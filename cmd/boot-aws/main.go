package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/osbuild/image-builder/internal/test"
	"github.com/osbuild/image-builder/pkg/cloud/awscloud"
)

// exitCheck can be deferred from the top of command functions to exit with an
// error code after any other defers are run in the same scope.
func exitCheck(err error) {
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error()+"\n")
		os.Exit(1)
	}
}

// resources created or allocated for an instance that can be cleaned up when
// tearing down.
type resources struct {
	Region        string   `json:"region,omitempty"`
	SecurityGroup *string  `json:"security-group,omitempty"`
	InstanceIDs   []string `json:"instances,omitempty"`
}

func saveResources(filename string, res *resources) error {
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0600)
}

func newClientFromArgs(flags *pflag.FlagSet, region string) (*awscloud.AWS, error) {
	if flags.Changed("access-key-id") {
		keyID, err := flags.GetString("access-key-id")
		if err != nil {
			return nil, err
		}
		secretKey, err := flags.GetString("secret-access-key")
		if err != nil {
			return nil, err
		}
		sessionToken, err := flags.GetString("session-token")
		if err != nil {
			return nil, err
		}

		return awscloud.New(region, keyID, secretKey, sessionToken)
	} else {
		return awscloud.NewDefault(region, "")
	}
}

func doSetup(a *awscloud.AWS, flags *pflag.FlagSet, res *resources, count int32, efa bool) error {
	if count < 1 {
		return fmt.Errorf("instances must be at least 1")
	}
	userDataFile, err := flags.GetString("user-data")
	if err != nil {
		return err
	}

	userData, err := os.ReadFile(userDataFile)
	if err != nil {
		return fmt.Errorf("cannot read user-data file %q: %w", userDataFile, err)
	}

	ami, err := flags.GetString("ami")
	if err != nil {
		return err
	}

	vpcID, err := flags.GetString("vpc-id")
	if err != nil {
		return err
	}
	subnetID, err := flags.GetString("subnet-id")
	if err != nil {
		return err
	}
	instanceType, err := flags.GetString("instance-type")
	if err != nil {
		return err
	}
	if efa && subnetID == "" {
		return fmt.Errorf("EFA requires --subnet-id")
	}

	fmt.Printf("Using AMI: %s\n", ami)

	securityGroupName := fmt.Sprintf("image-boot-tests-%s", uuid.New().String())
	securityGroup, err := a.CreateSecurityGroupEC2(securityGroupName, "image-tests-security-group", vpcID)
	if err != nil {
		return fmt.Errorf("CreateSecurityGroup(): %s", err.Error())
	}
	if securityGroup == nil || securityGroup.GroupId == nil || *securityGroup.GroupId == "" {
		return fmt.Errorf("CreateSecurityGroupEC2() returned no security group ID")
	}
	securityGroupID := *securityGroup.GroupId

	res.SecurityGroup = &securityGroupID

	_, err = a.AuthorizeSecurityGroupIngressEC2(securityGroupID, "0.0.0.0/0", 22, 22, "tcp")
	if err != nil {
		return fmt.Errorf("AuthorizeSecurityGroupIngressEC2(): %s", err.Error())
	}
	if efa {
		if err := a.AuthorizeSecurityGroupSelfEC2(securityGroupID); err != nil {
			return fmt.Errorf("AuthorizeSecurityGroupSelfEC2(): %w", err)
		}
	}

	ids, err := a.LaunchInstancesEC2(ami, securityGroupID, string(userData), instanceType, subnetID, count, efa)
	if err != nil {
		return fmt.Errorf("LaunchInstancesEC2(): %w", err)
	}
	res.InstanceIDs = append(res.InstanceIDs, ids...)
	if err := a.WaitInstanceRunningEC2(res.InstanceIDs...); err != nil {
		return fmt.Errorf("instances did not become running: %w", err)
	}
	for _, id := range res.InstanceIDs {
		ip, err := a.GetInstanceAddress(id)
		if err != nil {
			return fmt.Errorf("GetInstanceAddress(%s): %w", id, err)
		}
		fmt.Printf("Instance %s is running and has IP address %s\n", id, ip)
	}
	return nil
}

func setup(cmd *cobra.Command, args []string) {
	var fnerr error
	defer func() { exitCheck(fnerr) }()

	flags := cmd.Flags()

	region, err := flags.GetString("region")
	if err != nil {
		fnerr = err
		return
	}
	a, err := newClientFromArgs(flags, region)
	if err != nil {
		fnerr = err
		return
	}

	resourcesFile, err := flags.GetString("resourcefile")
	if err != nil {
		fnerr = err
		return
	}
	res := &resources{Region: region}
	count, err := flags.GetInt32("instances")
	if err != nil {
		fnerr = err
		return
	}
	efa, err := flags.GetBool("efa")
	if err != nil {
		fnerr = err
		return
	}
	fnerr = doSetup(a, flags, res, count, efa)
	if fnerr != nil {
		fmt.Fprintf(os.Stderr, "setup() failed: %s\n", fnerr.Error())
		fmt.Fprint(os.Stderr, "tearing down resources\n")
		tderr := doTeardown(a, res)
		if tderr != nil {
			fmt.Fprintf(os.Stderr, "teardown(): %s\n", tderr.Error())
			fnerr = errors.Join(fnerr, tderr)
		}
	}

	// Save the created resources, or any left over after failed setup and cleanup.
	if err := saveResources(resourcesFile, res); err != nil {
		fnerr = errors.Join(fnerr, fmt.Errorf("failed to write resources file: %w", err))
	}
	if fnerr != nil {
		return
	}
	fmt.Printf("IDs for any newly created resources are stored in %s. Use the teardown command to clean them up.\n", resourcesFile)
}

func doTeardown(aws *awscloud.AWS, res *resources) error {
	if len(res.InstanceIDs) > 0 {
		fmt.Printf("terminating instances %v\n", res.InstanceIDs)
		if _, err := aws.TerminateInstancesEC2(res.InstanceIDs, time.Hour); err != nil {
			return fmt.Errorf("failed to terminate instances: %w", err)
		}
		res.InstanceIDs = nil
	}

	if res.SecurityGroup != nil {
		fmt.Printf("deleting security group %s\n", *res.SecurityGroup)
		if _, err := aws.DeleteSecurityGroupEC2(*res.SecurityGroup); err != nil {
			return fmt.Errorf("cannot delete the security group: %v", err)
		}
		res.SecurityGroup = nil
	}

	return nil
}

func teardown(cmd *cobra.Command, args []string) {
	var fnerr error
	defer func() { exitCheck(fnerr) }()

	flags := cmd.Flags()

	resourcesFile, err := flags.GetString("resourcefile")
	if err != nil {
		fnerr = err
		return
	}

	resdata, err := os.ReadFile(resourcesFile)
	if err != nil {
		fnerr = fmt.Errorf("failed to read resources file: %s", err.Error())
		return
	}
	res := &resources{}
	if err := json.Unmarshal(resdata, res); err != nil {
		fnerr = fmt.Errorf("failed to unmarshal resources data: %s", err.Error())
		return
	}

	a, err := newClientFromArgs(flags, res.Region)
	if err != nil {
		fnerr = err
		return
	}

	fnerr = doTeardown(a, res)
	fnerr = errors.Join(fnerr, saveResources(resourcesFile, res))
}

func doRunExec(a *awscloud.AWS, command []string, flags *pflag.FlagSet, res *resources) error {
	privKey, err := flags.GetString("ssh-privkey")
	if err != nil {
		return err
	}
	username, err := flags.GetString("username")
	if err != nil {
		return err
	}

	tmpdir, err := os.MkdirTemp("", "boot-test-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	hostsfile := filepath.Join(tmpdir, "known_hosts")
	ip, err := a.GetInstanceAddress(res.InstanceIDs[0])
	if err != nil {
		return err
	}
	if err := test.Keyscan(ip, hostsfile); err != nil {
		return err
	}

	// ssh into the remote machine and exit immediately to check connection
	if err := test.SshRun(ip, username, privKey, hostsfile, "exit"); err != nil {
		return err
	}

	// copy every argument that is a file to the remote host (basename only)
	// and construct remote command
	// NOTE: this wont work with directories or with multiple args in different
	// paths that share the same basename - it's very limited
	remoteCommand := make([]string, len(command))
	for idx := range command {
		arg := command[idx]
		if test.IsRegularFile(arg) {
			// scp the file and add it to the remote command by its base name
			remotePath := filepath.Base(arg)
			remoteCommand[idx] = remotePath
			if err := test.ScpFile(ip, username, privKey, hostsfile, arg, remotePath); err != nil {
				return err
			}
		} else {
			// not a file: add the arg as is
			remoteCommand[idx] = arg
		}
	}

	// add ./ to first element for the executable
	remoteCommand[0] = fmt.Sprintf("./%s", remoteCommand[0])

	// run the executable
	return test.SshRun(ip, username, privKey, hostsfile, remoteCommand...)
}

func runExec(cmd *cobra.Command, args []string) {
	var fnerr error
	defer func() { exitCheck(fnerr) }()

	command := args
	flags := cmd.Flags()

	region, fnerr := flags.GetString("region")
	if fnerr != nil {
		return
	}
	a, fnerr := newClientFromArgs(flags, region)
	if fnerr != nil {
		return
	}

	res := &resources{}
	defer func() {
		tderr := doTeardown(a, res)
		if tderr != nil {
			// report it but let the exitCheck() handle fnerr
			fmt.Fprintf(os.Stderr, "teardown(): %s\n", tderr.Error())
		}
	}()

	fnerr = doSetup(a, flags, res, 1, false)
	if fnerr != nil {
		return
	}

	fnerr = doRunExec(a, command, flags, res)
}

func setupCLI() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:                   "boot",
		Long:                  "boot an image on AWS EC2",
		DisableFlagsInUseLine: true,
	}

	rootFlags := rootCmd.PersistentFlags()
	rootFlags.String("access-key-id", "", "access key ID")
	rootFlags.String("secret-access-key", "", "secret access key")
	rootFlags.String("session-token", "", "session token")

	setupCmd := &cobra.Command{
		Use:                   "setup [--resourcefile <filename>]",
		Short:                 "boot an AMI and save the created resource IDs to a file for later teardown",
		Args:                  cobra.NoArgs,
		Run:                   setup,
		DisableFlagsInUseLine: true,
	}
	setupCmd.Flags().String("region", "", "target region")
	setupCmd.Flags().String("ami", "", "AMI ID to boot")
	setupCmd.Flags().String("vpc-id", "", "ID of the VPC to use")
	setupCmd.Flags().String("subnet-id", "", "ID of the subnet to launch the instance in")
	setupCmd.Flags().StringP("resourcefile", "r", "resources.json", "path to store the resource IDs")
	setupCmd.Flags().String("user-data", "", "path to a cloud-init user-data file (passed through unchanged)")
	setupCmd.Flags().String("instance-type", "", "instance type to use, e.g. t3.small (x86_64) or t4g.medium (aarch64)")
	setupCmd.Flags().Int32("instances", 1, "number of instances to launch")
	setupCmd.Flags().Bool("efa", false, "attach an EFA primary network interface and allow intra-group traffic")
	setupCmd.MarkFlagsRequiredTogether("vpc-id", "subnet-id")
	for _, flag := range []string{"region", "ami", "instance-type"} {
		exitCheck(setupCmd.MarkFlagRequired(flag))
	}
	rootCmd.AddCommand(setupCmd)

	teardownCmd := &cobra.Command{
		Use:   "teardown [--resourcefile <filename>]",
		Short: "teardown (clean up) all the resources specified in a resources file created by a previous 'setup' call",
		Args:  cobra.NoArgs,
		Run:   teardown,
	}
	teardownCmd.Flags().StringP("resourcefile", "r", "resources.json", "path containing the resource IDs")
	rootCmd.AddCommand(teardownCmd)

	runCmd := &cobra.Command{
		Use:   "run <executable>...",
		Short: "boot an AMI, then upload the specified executable and run it on the remote host",
		Long:  "boot an AMI on AWS EC2, then upload the executable file specified by the first positional argument and execute it via SSH with the args on the command line",
		Args:  cobra.MinimumNArgs(1),
		Run:   runExec,
	}
	runCmd.Flags().String("region", "", "target region")
	runCmd.Flags().String("ami", "", "AMI ID to boot")
	runCmd.Flags().String("username", "", "name of the user to create on the system")
	runCmd.Flags().String("ssh-privkey", "", "path to user's private ssh key")
	runCmd.Flags().String("vpc-id", "", "ID of the VPC to use")
	runCmd.Flags().String("subnet-id", "", "ID of the subnet to launch the instance in")
	runCmd.Flags().String("user-data", "", "path to a cloud-init user-data file (passed through unchanged)")
	runCmd.Flags().String("instance-type", "", "instance type to use, e.g. t3.small (x86_64) or t4g.medium (aarch64)")
	runCmd.MarkFlagsRequiredTogether("vpc-id", "subnet-id")
	for _, flag := range []string{"region", "ami", "instance-type", "username", "ssh-privkey"} {
		exitCheck(runCmd.MarkFlagRequired(flag))
	}
	rootCmd.AddCommand(runCmd)

	return rootCmd
}

func main() {
	cmd := setupCLI()
	exitCheck(cmd.Execute())
}
