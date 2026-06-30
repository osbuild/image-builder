package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/osbuild/image-builder/internal/test"
	"github.com/osbuild/image-builder/pkg/cloud/azure"
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
	VM *azure.VM `json:"vm,omitempty"`
}

func newClientFromArgs(flags *pflag.FlagSet) (*azure.Client, error) {
	client, err := flags.GetString("client-id")
	if err != nil {
		return nil, err
	}
	secret, err := flags.GetString("client-secret")
	if err != nil {
		return nil, err
	}
	tenant, err := flags.GetString("tenant")
	if err != nil {
		return nil, err
	}
	subscr, err := flags.GetString("subscription")
	if err != nil {
		return nil, err
	}

	return azure.NewClient(
		azure.Credentials{
			ClientID:     client,
			ClientSecret: secret,
		},
		tenant,
		subscr,
	)
}

func getDefaultSize(architecture string) (string, error) {
	switch architecture {
	case "x86_64":
		return "Standard_D2ls_v5", nil
	case "aarch64":
		return "Standard_D2pls_v5", nil
	default:
		return "", fmt.Errorf("getDefaultSize(): unknown architecture %q", architecture)
	}
}

// Assume that any image launched from a snapshot is a windows image. Booting from a snapshot is
// used for the WSL test case.
func isWindows(flags *pflag.FlagSet) (bool, error) {
	snapshot, err := flags.GetString("snapshot")
	if err != nil {
		return false, err
	}
	return snapshot != "", err
}

func doSetup(ac *azure.Client, flags *pflag.FlagSet, res *resources) error {
	rg, err := flags.GetString("resource-group")
	if err != nil {
		return err
	}

	image, err := flags.GetString("image")
	if err != nil {
		return err
	}

	snapshot, err := flags.GetString("snapshot")
	if err != nil {
		return err
	}

	if image == "" && snapshot == "" {
		return fmt.Errorf("either --image or --snapshot must be provided")
	}

	architecture, err := flags.GetString("arch")
	if err != nil {
		return err
	}

	windows, err := isWindows(flags)
	if err != nil {
		return err
	}

	vmName, err := flags.GetString("vm-name")
	if err != nil {
		return err
	}

	size, err := flags.GetString("size")
	if err != nil {
		return err
	}
	if size == "" {
		size, err = getDefaultSize(architecture)
		if err != nil {
			return err
		}
	}

	username, err := flags.GetString("username")
	if err != nil {
		return err
	}

	keyPath, err := flags.GetString("ssh-pubkey")
	if err != nil {
		return err
	}

	keyfile, err := os.Open(keyPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := keyfile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "unable to close public key file: %s\n", err.Error())
		}
	}()

	keyData, err := io.ReadAll(keyfile)
	if err != nil {
		return err
	}

	ctx := context.Background()
	vm, err := ac.CreateVM(
		ctx,
		rg,
		azure.VMOptions{
			Name:     vmName,
			Image:    image,
			Snapshot: snapshot,
			Size:     size,
			User:     username,
			SSHKey:   string(keyData),
			Windows:  windows,
		},
	)
	if err != nil {
		return err
	}
	res.VM = vm
	return nil
}

func setup(cmd *cobra.Command, args []string) {
	var fnerr error
	defer func() { exitCheck(fnerr) }()

	flags := cmd.Flags()

	ac, err := newClientFromArgs(flags)
	if err != nil {
		fnerr = err
		return
	}

	res := &resources{}
	fnerr = doSetup(ac, flags, res)
	if fnerr != nil {
		fmt.Fprintf(os.Stderr, "setup() failed: %s\n", fnerr.Error())
		fmt.Fprint(os.Stderr, "tearing down resources\n")

		if err := doTeardown(ac, res); err != nil {
			fnerr = fmt.Errorf("failed to tear down resources: %s", err.Error())
			return
		}
	}

	resourcesFile, err := flags.GetString("resourcefile")
	if err != nil {
		fnerr = err
		return
	}
	resfile, err := os.Create(resourcesFile)
	if err != nil {
		fnerr = err
		return
	}
	defer func() {
		if err := resfile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "unable to close resources file: %s\n", err.Error())
		}
	}()

	resdata, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		fnerr = err
		return
	}
	_, err = resfile.Write(resdata)
	if err != nil {
		fnerr = err
		return
	}
}

func doTeardown(ac *azure.Client, res *resources) error {
	ctx := context.Background()

	if res.VM != nil {
		if err := ac.DestroyVM(ctx, res.VM); err != nil {
			return err
		}
	}

	return nil
}

func teardown(cmd *cobra.Command, args []string) {
	var fnerr error
	defer func() { exitCheck(fnerr) }()

	flags := cmd.Flags()
	ac, err := newClientFromArgs(flags)
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
	defer func() {
		if err := resfile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "unable to close resources file: %s\n", err.Error())
		}
	}()

	resdata, err := io.ReadAll(resfile)
	if err != nil {
		fnerr = fmt.Errorf("failed to read resources file: %s", err.Error())
		return
	}
	if err := json.Unmarshal(resdata, res); err != nil {
		fnerr = fmt.Errorf("failed to unmarshal resources data: %s", err.Error())
		return
	}

	fnerr = doTeardown(ac, res)
}

func doRunExec(command []string, flags *pflag.FlagSet, res *resources) error {
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
	ip := res.VM.IPAddress
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

	windows, err := isWindows(flags)
	if err != nil {
		return err
	}
	// on windows the (wsl) image should be scp'd to the host so it can be imported into wsl
	if windows {
		localImage, err := flags.GetString("local-image")
		if err != nil {
			return err
		}
		if localImage == "" {
			return fmt.Errorf("--local-image is required when using --snapshot")
		}
		remotePath := filepath.Base(localImage)
		if err := test.ScpFile(ip, username, privKey, hostsfile, localImage, remotePath); err != nil {
			return err
		}
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

	// batch (cmd.exe) cannot interpret ./
	if !windows {
		remoteCommand[0] = fmt.Sprintf("./%s", remoteCommand[0])
	}

	// run the executable
	return test.SshRun(ip, username, privKey, hostsfile, remoteCommand...)
}

func runExec(cmd *cobra.Command, args []string) {
	var fnerr error
	defer func() { exitCheck(fnerr) }()

	command := args
	flags := cmd.Flags()

	ac, fnerr := newClientFromArgs(flags)
	if fnerr != nil {
		return
	}

	res := &resources{}
	defer func() {
		if err := doTeardown(ac, res); err != nil {
			fnerr = fmt.Errorf("failed to destroy vm: %s", err.Error())
			return
		}
	}()

	fnerr = doSetup(ac, flags, res)
	if fnerr != nil {
		return
	}

	fnerr = doRunExec(command, flags, res)
}

func setupCLI() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:                   "boot",
		Long:                  "boot an image on Azure",
		DisableFlagsInUseLine: true,
	}

	rootFlags := rootCmd.PersistentFlags()
	rootFlags.String("client-id", "", "client ID")
	rootFlags.String("client-secret", "", "client secret")
	rootFlags.String("tenant", "", "tenant id")
	rootFlags.String("subscription", "", "subscription")
	rootFlags.String("resource-group", "", "resource group of image and vm")
	rootFlags.String("username", "azure", "name of the user to create on the system")
	rootFlags.String("ssh-pubkey", "", "path to user's public ssh key, must be an rsa key")
	rootFlags.String("ssh-privkey", "", "path to user's private ssh key")
	rootFlags.String("image", "", "full resource ID of the remote image, should already exist in the resource group")
	rootFlags.String("snapshot", "", "full resource ID of the snapshot, assumed to be a windows image for WSL testing")
	rootFlags.String("local-image", "", "path to local image to SCP to the VM (only for WSL with --snapshot)")
	rootFlags.String("vm-name", "vm-name", "name of the VM to create, all dependencies will be prefixed with this name")
	rootFlags.String("size", "", "size or instance type of the VM to create")
	rootFlags.String("arch", "x86_64", "architecture (x86_64 or aarch64)")

	exitCheck(rootCmd.MarkPersistentFlagRequired("client-id"))
	exitCheck(rootCmd.MarkPersistentFlagRequired("client-secret"))
	exitCheck(rootCmd.MarkPersistentFlagRequired("tenant"))
	exitCheck(rootCmd.MarkPersistentFlagRequired("subscription"))

	setupCmd := &cobra.Command{
		Use:                   "setup [--resourcefile <filename>]",
		Short:                 "boot an image and save the created resource IDs to a file for later teardown",
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
		Short: "boot an image, then upload the specified executable and run it on the remote host",
		Long:  "boot an image on Azure, then upload the executable file specified by the first positional argument and execute it via SSH with the args on the command line",
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
