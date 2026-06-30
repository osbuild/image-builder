package main

import (
	"encoding/json"
	"fmt"
	"io"
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

// createUserData creates cloud-init's user-data that contains user redhat with
// the specified public key
func createUserData(username, publicKeyFile string) (string, error) {
	publicKey, err := os.ReadFile(publicKeyFile)
	if err != nil {
		return "", err
	}

	userData := fmt.Sprintf(`#cloud-config
user: %s
ssh_authorized_keys:
  - %s
`, username, string(publicKey))

	return userData, nil
}

// resources created or allocated for an instance that can be cleaned up when
// tearing down.
type resources struct {
	SecurityGroup *string `json:"security-group,omitempty"`
	InstanceID    *string `json:"instance,omitempty"`
}

func getInstanceType(arch string) (string, error) {
	switch arch {
	case "x86_64":
		return "t3.small", nil
	case "aarch64":
		return "t4g.medium", nil
	default:
		return "", fmt.Errorf("getInstanceType(): unknown architecture %q", arch)
	}
}

func newClientFromArgs(flags *pflag.FlagSet) (*awscloud.AWS, error) {
	region, err := flags.GetString("region")
	if err != nil {
		return nil, err
	}
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
}

func doSetup(a *awscloud.AWS, flags *pflag.FlagSet, res *resources) error {
	username, err := flags.GetString("username")
	if err != nil {
		return err
	}
	sshPubKey, err := flags.GetString("ssh-pubkey")
	if err != nil {
		return err
	}

	userData, err := createUserData(username, sshPubKey)
	if err != nil {
		return fmt.Errorf("createUserData(): %s", err.Error())
	}

	ami, err := flags.GetString("ami")
	if err != nil {
		return err
	}

	archArg, err := flags.GetString("arch")
	if err != nil {
		return err
	}

	fmt.Printf("Using AMI: %s\n", ami)

	securityGroupName := fmt.Sprintf("image-boot-tests-%s", uuid.New().String())
	securityGroup, err := a.CreateSecurityGroupEC2(securityGroupName, "image-tests-security-group")
	if err != nil {
		return fmt.Errorf("CreateSecurityGroup(): %s", err.Error())
	}

	res.SecurityGroup = securityGroup.GroupId

	_, err = a.AuthorizeSecurityGroupIngressEC2(*securityGroup.GroupId, "0.0.0.0/0", 22, 22, "tcp")
	if err != nil {
		return fmt.Errorf("AuthorizeSecurityGroupIngressEC2(): %s", err.Error())
	}

	instance, err := getInstanceType(archArg)
	if err != nil {
		return err
	}
	runResult, err := a.RunInstanceEC2(ami, *securityGroup.GroupId, userData, instance)
	if err != nil {
		return fmt.Errorf("RunInstanceEC2(): %s", err.Error())
	}
	instanceID := runResult.Instances[0].InstanceId
	res.InstanceID = instanceID

	ip, err := a.GetInstanceAddress(*instanceID)
	if err != nil {
		return fmt.Errorf("GetInstanceAddress(): %s", err.Error())
	}
	fmt.Printf("Instance %s is running and has IP address %s\n", *instanceID, ip)
	return nil
}

func setup(cmd *cobra.Command, args []string) {
	var fnerr error
	defer func() { exitCheck(fnerr) }()

	flags := cmd.Flags()

	a, err := newClientFromArgs(flags)
	if err != nil {
		fnerr = err
		return
	}

	// collect resources into res and write them out when the function returns
	resourcesFile, err := flags.GetString("resourcefile")
	if err != nil {
		fnerr = err
		return
	}
	res := &resources{}

	fnerr = doSetup(a, flags, res)
	if fnerr != nil {
		fmt.Fprintf(os.Stderr, "setup() failed: %s\n", fnerr.Error())
		fmt.Fprint(os.Stderr, "tearing down resources\n")
		tderr := doTeardown(a, res)
		if tderr != nil {
			fmt.Fprintf(os.Stderr, "teardown(): %s\n", tderr.Error())
		}
	}

	resdata, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		fnerr = fmt.Errorf("failed to marshal resources data: %s", err.Error())
		return
	}
	resfile, err := os.Create(resourcesFile)
	if err != nil {
		fnerr = fmt.Errorf("failed to create resources file: %s", err.Error())
		return
	}
	_, err = resfile.Write(resdata)
	if err != nil {
		fnerr = fmt.Errorf("failed to write resources file: %s", err.Error())
		return
	}
	fmt.Printf("IDs for any newly created resources are stored in %s. Use the teardown command to clean them up.\n", resourcesFile)
	if err = resfile.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "error closing resources file: %s\n", err.Error())
		fnerr = err
		return
	}
}

func doTeardown(aws *awscloud.AWS, res *resources) error {
	if res.InstanceID != nil {
		fmt.Printf("terminating instance %s\n", *res.InstanceID)
		if _, err := aws.TerminateInstancesEC2([]string{*res.InstanceID}, time.Hour); err != nil {
			return fmt.Errorf("failed to terminate instance: %v", err)
		}
	}

	if res.SecurityGroup != nil {
		fmt.Printf("deleting security group %s\n", *res.SecurityGroup)
		if _, err := aws.DeleteSecurityGroupEC2(*res.SecurityGroup); err != nil {
			return fmt.Errorf("cannot delete the security group: %v", err)
		}
	}

	return nil
}

func teardown(cmd *cobra.Command, args []string) {
	var fnerr error
	defer func() { exitCheck(fnerr) }()

	flags := cmd.Flags()

	a, err := newClientFromArgs(flags)
	if err != nil {
		fnerr = err
		return
	}

	resourcesFile, err := flags.GetString("resourcefile")
	if err != nil {
		return
	}

	res := &resources{}
	resfile, err := os.Open(resourcesFile)
	if err != nil {
		fnerr = fmt.Errorf("failed to open resources file: %s", err.Error())
		return
	}
	resdata, err := io.ReadAll(resfile)
	if err != nil {
		fnerr = fmt.Errorf("failed to read resources file: %s", err.Error())
		return
	}
	if err := json.Unmarshal(resdata, res); err != nil {
		fnerr = fmt.Errorf("failed to unmarshal resources data: %s", err.Error())
		return
	}

	fnerr = doTeardown(a, res)
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
	ip, err := a.GetInstanceAddress(*res.InstanceID)
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

	isFile := func(path string) bool {
		fileInfo, err := os.Stat(path)
		if err != nil {
			// ignore error and assume it's not a path
			return false
		}

		// Check if it's a regular file
		return fileInfo.Mode().IsRegular()
	}

	// copy every argument that is a file to the remote host (basename only)
	// and construct remote command
	// NOTE: this wont work with directories or with multiple args in different
	// paths that share the same basename - it's very limited
	remoteCommand := make([]string, len(command))
	for idx := range command {
		arg := command[idx]
		if isFile(arg) {
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

	a, fnerr := newClientFromArgs(flags)
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

	fnerr = doSetup(a, flags, res)
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
	rootFlags.String("region", "", "target region")
	rootFlags.String("ami", "", "AMI ID to boot")
	rootFlags.String("arch", "", "arch (x86_64 or aarch64)")
	rootFlags.String("username", "", "name of the user to create on the system")
	rootFlags.String("ssh-pubkey", "", "path to user's public ssh key")
	rootFlags.String("ssh-privkey", "", "path to user's private ssh key")

	exitCheck(rootCmd.MarkPersistentFlagRequired("access-key-id"))
	exitCheck(rootCmd.MarkPersistentFlagRequired("secret-access-key"))
	exitCheck(rootCmd.MarkPersistentFlagRequired("region"))
	exitCheck(rootCmd.MarkPersistentFlagRequired("ami"))
	exitCheck(rootCmd.MarkPersistentFlagRequired("arch"))

	// TODO: make it optional and use a default
	exitCheck(rootCmd.MarkPersistentFlagRequired("username"))

	// TODO: make ssh key pair optional for 'run' and if not specified generate
	// a temporary key pair
	exitCheck(rootCmd.MarkPersistentFlagRequired("ssh-privkey"))
	exitCheck(rootCmd.MarkPersistentFlagRequired("ssh-pubkey"))

	setupCmd := &cobra.Command{
		Use:                   "setup [--resourcefile <filename>]",
		Short:                 "boot an AMI and save the created resource IDs to a file for later teardown",
		Args:                  cobra.NoArgs,
		Run:                   setup,
		DisableFlagsInUseLine: true,
	}
	setupCmd.Flags().StringP("resourcefile", "r", "resources.json", "path to store the resource IDs")
	rootCmd.AddCommand(setupCmd)

	teardownCmd := &cobra.Command{
		Use:   "teardown [--resourcefile <filename>]",
		Short: "teardown (clean up) all the resources specified in a resources file created by a previous 'setup' call",
		Args:  cobra.NoArgs,
		Run:   teardown,
	}
	teardownCmd.Flags().StringP("resourcefile", "r", "resources.json", "path to store the resource IDs")
	rootCmd.AddCommand(teardownCmd)

	runCmd := &cobra.Command{
		Use:   "run <executable>...",
		Short: "boot an AMI, then upload the specified executable and run it on the remote host",
		Long:  "boot an AMI on AWS EC2, then upload the executable file specified by the first positional argument and execute it via SSH with the args on the command line",
		Args:  cobra.MinimumNArgs(1),
		Run:   runExec,
	}
	rootCmd.AddCommand(runCmd)

	return rootCmd
}

func main() {
	cmd := setupCLI()
	exitCheck(cmd.Execute())
}
