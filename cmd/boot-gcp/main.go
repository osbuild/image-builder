package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/osbuild/image-builder/internal/test"
	"github.com/osbuild/image-builder/pkg/cloud/gcp"
)

// exitCheck can be deferred from the top of command functions to exit with an
// error code after any other defers are run in the same scope.
func exitCheck(err error) {
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error()+"\n")
		os.Exit(1)
	}
}

type resources struct {
	Instance *gcp.BootInstance `json:"instance,omitempty"`
}

func newClientFromArgs(flags *pflag.FlagSet) (*gcp.GCP, error) {
	var client *gcp.GCP
	var err error
	if flags.Changed("credentials") {
		path, err := flags.GetString("credentials")
		if err != nil {
			return nil, err
		}
		client, err = gcp.NewFromFile(path)
		if err != nil {
			return nil, err
		}
	} else {
		client, err = gcp.New(nil)
		if err != nil {
			return nil, err
		}
	}
	project, err := flags.GetString("project")
	if err != nil {
		return nil, err
	}
	if project != "" {
		client.SetProjectID(project)
	}
	return client, nil
}

func readSSHKeys(flags *pflag.FlagSet) (string, error) {
	sshKeysFile, err := flags.GetString("ssh-authorized-keys-file")
	if err != nil {
		return "", err
	}
	username, err := flags.GetString("username")
	if err != nil {
		return "", err
	}
	keys, err := os.ReadFile(sshKeysFile)
	if err != nil {
		return "", fmt.Errorf("cannot read ssh-authorized-keys-file %q: %w", sshKeysFile, err)
	}
	return gcp.FormatSSHKeysMetadata(username, string(keys))
}

func doSetup(client *gcp.GCP, flags *pflag.FlagSet, res *resources) error {
	ctx := context.Background()

	zone, err := flags.GetString("zone")
	if err != nil {
		return err
	}
	image, err := flags.GetString("image")
	if err != nil {
		return err
	}
	arch, err := flags.GetString("arch")
	if err != nil {
		return err
	}
	machineType, err := flags.GetString("machine-type")
	if err != nil {
		return err
	}
	if machineType == "" {
		machineType, err = gcp.DefaultMachineType(arch)
		if err != nil {
			return err
		}
	}
	network, err := flags.GetString("network")
	if err != nil {
		return err
	}
	subnet, err := flags.GetString("subnet")
	if err != nil {
		return err
	}
	instanceName, err := flags.GetString("instance-name")
	if err != nil {
		return err
	}
	sshKeys, err := readSSHKeys(flags)
	if err != nil {
		return err
	}

	if client.GetProjectID() == "" {
		return fmt.Errorf("GCP project is unknown: set --project or use credentials that include project_id")
	}

	fmt.Printf("Using image: %s\n", image)

	inst, err := client.LaunchBootInstance(ctx, zone, gcp.BootInstanceOptions{
		ImageName:    image,
		MachineType:  machineType,
		SSHKeys:      sshKeys,
		Network:      network,
		Subnetwork:   subnet,
		InstanceName: instanceName,
	})
	if err != nil {
		return err
	}
	res.Instance = inst
	fmt.Printf("Instance %s is running and has IP address %s\n", inst.Name, inst.PublicIP)
	return nil
}

func setup(cmd *cobra.Command, args []string) {
	var fnerr error
	defer func() { exitCheck(fnerr) }()

	flags := cmd.Flags()
	client, err := newClientFromArgs(flags)
	if err != nil {
		fnerr = err
		return
	}

	resourcesFile, err := flags.GetString("resourcefile")
	if err != nil {
		fnerr = err
		return
	}
	res := &resources{}

	fnerr = doSetup(client, flags, res)
	if fnerr != nil {
		fmt.Fprintf(os.Stderr, "setup() failed: %s\n", fnerr.Error())
		fmt.Fprint(os.Stderr, "tearing down resources\n")
		if res.Instance != nil {
			tderr := doTeardown(client, res)
			if tderr != nil {
				fmt.Fprintf(os.Stderr, "teardown(): %s\n", tderr.Error())
			}
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

func doTeardown(client *gcp.GCP, res *resources) error {
	if res.Instance == nil {
		return nil
	}
	fmt.Printf("deleting instance %s\n", res.Instance.Name)
	return client.DeleteBootInstance(context.Background(), res.Instance)
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

	client, err := newClientFromArgs(flags)
	if err != nil {
		fnerr = err
		return
	}
	if client.GetProjectID() == "" && res.Instance != nil && res.Instance.Project != "" {
		client.SetProjectID(res.Instance.Project)
	}
	if client.GetProjectID() == "" {
		fnerr = fmt.Errorf("GCP project is unknown: set --project or use a resources file from setup")
		return
	}

	fnerr = doTeardown(client, res)
}

func doRunExec(client *gcp.GCP, command []string, flags *pflag.FlagSet, res *resources) error {
	privKey, err := flags.GetString("ssh-privkey")
	if err != nil {
		return err
	}
	username, err := flags.GetString("username")
	if err != nil {
		return err
	}
	if res.Instance == nil || res.Instance.PublicIP == "" {
		return fmt.Errorf("no instance public IP in resources")
	}

	tmpdir, err := os.MkdirTemp("", "boot-test-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	hostsfile := filepath.Join(tmpdir, "known_hosts")
	ip := res.Instance.PublicIP
	if err := test.Keyscan(ip, hostsfile); err != nil {
		return err
	}
	if err := test.SshRun(ip, username, privKey, hostsfile, "exit"); err != nil {
		return err
	}

	isFile := func(path string) bool {
		fileInfo, err := os.Stat(path)
		if err != nil {
			return false
		}
		return fileInfo.Mode().IsRegular()
	}

	remoteCommand := make([]string, len(command))
	for idx := range command {
		arg := command[idx]
		if isFile(arg) {
			remotePath := filepath.Base(arg)
			remoteCommand[idx] = remotePath
			if err := test.ScpFile(ip, username, privKey, hostsfile, arg, remotePath); err != nil {
				return err
			}
		} else {
			remoteCommand[idx] = arg
		}
	}
	remoteCommand[0] = fmt.Sprintf("./%s", remoteCommand[0])
	return test.SshRun(ip, username, privKey, hostsfile, remoteCommand...)
}

func runExec(cmd *cobra.Command, args []string) {
	var fnerr error
	defer func() { exitCheck(fnerr) }()

	command := args
	flags := cmd.Flags()
	client, fnerr := newClientFromArgs(flags)
	if fnerr != nil {
		return
	}

	res := &resources{}
	defer func() {
		tderr := doTeardown(client, res)
		if tderr != nil {
			fmt.Fprintf(os.Stderr, "teardown(): %s\n", tderr.Error())
		}
	}()

	fnerr = doSetup(client, flags, res)
	if fnerr != nil {
		return
	}

	fnerr = doRunExec(client, command, flags, res)
}

func setupCLI() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:                   "boot-gcp",
		Long:                  "boot an image on Google Compute Engine",
		DisableFlagsInUseLine: true,
	}

	rootFlags := rootCmd.PersistentFlags()
	rootFlags.String("credentials", "", "path to GCP service account JSON (optional; uses ADC otherwise)")

	setupCmd := &cobra.Command{
		Use:                   "setup [--resourcefile <filename>]",
		Short:                 "boot an image and save created resource IDs for teardown",
		Args:                  cobra.NoArgs,
		Run:                   setup,
		DisableFlagsInUseLine: true,
	}
	setupCmd.Flags().String("project", "", "GCP project ID (required for user ADC; optional if credentials include project_id)")
	setupCmd.Flags().String("zone", "", "GCP zone")
	setupCmd.Flags().String("image", "", "Compute Engine image name in the credentials project")
	setupCmd.Flags().String("arch", "", "arch (x86_64 or aarch64)")
	setupCmd.Flags().StringP("resourcefile", "r", "resources.json", "path to store resource IDs")
	setupCmd.Flags().String("ssh-authorized-keys-file", "", "path to SSH public key file")
	setupCmd.Flags().String("username", "", "SSH user name for GCE ssh-keys metadata")
	setupCmd.Flags().String("machine-type", "", "machine type (default depends on --arch)")
	setupCmd.Flags().String("network", "", "VPC network URL or name (default VPC if empty)")
	setupCmd.Flags().String("subnet", "", "subnet name or URL (required when the project has no default subnet in the zone region)")
	setupCmd.Flags().String("instance-name", "", "VM name (generated if empty)")
	for _, flag := range []string{"project", "zone", "subnet", "image", "arch", "ssh-authorized-keys-file", "username"} {
		exitCheck(setupCmd.MarkFlagRequired(flag))
	}
	rootCmd.AddCommand(setupCmd)

	teardownCmd := &cobra.Command{
		Use:   "teardown [--resourcefile <filename>]",
		Short: "delete resources listed in a setup resources file",
		Args:  cobra.NoArgs,
		Run:   teardown,
	}
	teardownCmd.Flags().StringP("resourcefile", "r", "resources.json", "path containing resource IDs")
	teardownCmd.Flags().String("project", "", "GCP project ID (optional if stored in the resources file)")
	rootCmd.AddCommand(teardownCmd)

	runCmd := &cobra.Command{
		Use:   "run <executable>...",
		Short: "boot an image, upload and run a command over SSH, then teardown",
		Args:  cobra.MinimumNArgs(1),
		Run:   runExec,
	}
	runCmd.Flags().String("project", "", "GCP project ID (required for user ADC; optional if credentials include project_id)")
	runCmd.Flags().String("zone", "", "GCP zone")
	runCmd.Flags().String("image", "", "Compute Engine image name in the credentials project")
	runCmd.Flags().String("arch", "", "arch (x86_64 or aarch64)")
	runCmd.Flags().String("username", "", "SSH user name")
	runCmd.Flags().String("ssh-privkey", "", "path to SSH private key")
	runCmd.Flags().String("ssh-authorized-keys-file", "", "path to SSH public key file")
	runCmd.Flags().String("machine-type", "", "machine type (default depends on --arch)")
	runCmd.Flags().String("network", "", "VPC network URL or name (default VPC if empty)")
	runCmd.Flags().String("subnet", "", "subnet name or URL (required when the project has no default subnet in the zone region)")
	runCmd.Flags().String("instance-name", "", "VM name (generated if empty)")
	for _, flag := range []string{"project", "zone", "subnet", "image", "arch", "username", "ssh-privkey", "ssh-authorized-keys-file"} {
		exitCheck(runCmd.MarkFlagRequired(flag))
	}
	rootCmd.AddCommand(runCmd)

	return rootCmd
}

func main() {
	cmd := setupCLI()
	exitCheck(cmd.Execute())
}
