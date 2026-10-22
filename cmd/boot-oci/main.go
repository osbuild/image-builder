package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/osbuild/image-builder/internal/test"
	ocicloud "github.com/osbuild/image-builder/pkg/cloud/oci"
)

// exitCheck can be deferred from the top of command functions to exit with an
// error code after any other defers are run in the same scope.
func exitCheck(err error) {
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error()+"\n")
		os.Exit(1)
	}
}

// resources created for an instance that can be cleaned up when tearing down.
type resources struct {
	Region     string  `json:"region,omitempty"`
	InstanceID *string `json:"instance,omitempty"`
	PublicIP   string  `json:"public-ip,omitempty"`
}

func doSetup(c *ocicloud.Client, flags *pflag.FlagSet, res *resources) error {
	ctx := context.Background()

	imageID, err := flags.GetString("image")
	if err != nil {
		return err
	}
	compartment, err := flags.GetString("compartment-id")
	if err != nil {
		return err
	}
	subnetID, err := flags.GetString("subnet-id")
	if err != nil {
		return err
	}
	arch, err := flags.GetString("arch")
	if err != nil {
		return err
	}
	shape, err := flags.GetString("shape")
	if err != nil {
		return err
	}
	availabilityDomain, err := flags.GetString("availability-domain")
	if err != nil {
		return err
	}
	shapeConfigJSON, err := flags.GetString("shape-config")
	if err != nil {
		return err
	}
	sshKeysFile, err := flags.GetString("ssh-authorized-keys-file")
	if err != nil {
		return err
	}

	opts := ocicloud.LaunchOptions{
		CompartmentID:      compartment,
		SubnetID:           subnetID,
		ImageID:            imageID,
		AvailabilityDomain: availabilityDomain,
	}
	if shape == "" {
		opts.Shape, opts.ShapeConfig, err = ocicloud.DefaultShape(arch)
		if err != nil {
			return err
		}
	} else {
		opts.Shape = shape
		// OCI flex shapes require ShapeConfig even when --shape is set explicitly.
		if shapeConfigJSON == "" && strings.HasSuffix(shape, ".Flex") {
			_, opts.ShapeConfig, err = ocicloud.DefaultShape(arch)
			if err != nil {
				return err
			}
		}
	}
	if shapeConfigJSON != "" {
		opts.ShapeConfig, err = ocicloud.ParseShapeConfig(shapeConfigJSON)
		if err != nil {
			return err
		}
	}
	if sshKeysFile != "" {
		keys, err := os.ReadFile(sshKeysFile)
		if err != nil {
			return fmt.Errorf("cannot read ssh-authorized-keys-file %q: %w", sshKeysFile, err)
		}
		opts.SSHAuthorizedKeys = string(keys)
	}

	if flagDebug(flags) {
		cliRegion, err := flags.GetString("region")
		if err != nil {
			return err
		}
		fmt.Printf("CLI --region: %s (API endpoint region: %s)\n", cliRegion, c.Region())
		ocicloud.LogLaunchRequest(c, opts)
	}

	instanceID, err := c.LaunchInstance(ctx, opts)
	if err != nil {
		return err
	}
	res.InstanceID = &instanceID

	ip, err := c.InstancePublicIP(ctx, compartment, instanceID)
	if err != nil {
		return err
	}
	res.PublicIP = ip
	fmt.Printf("Instance %s is running and has IP address %s\n", instanceID, ip)
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
	client, err := ocicloud.NewClient(region)
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

	fnerr = doSetup(client, flags, res)
	if fnerr != nil {
		fmt.Fprintf(os.Stderr, "setup() failed: %s\n", fnerr.Error())
		fmt.Fprint(os.Stderr, "tearing down resources\n")
		if tderr := doTeardown(client, res); tderr != nil {
			fmt.Fprintf(os.Stderr, "teardown(): %s\n", tderr.Error())
		}
		return // skip resources file when setup did not succeed
	}

	resdata, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		fnerr = fmt.Errorf("failed to marshal resources data: %w", err)
		return
	}
	resfile, err := os.Create(resourcesFile)
	if err != nil {
		fnerr = fmt.Errorf("failed to create resources file: %w", err)
		return
	}
	// Ensure the resources file handle is released if Write fails.
	defer func() {
		if cerr := resfile.Close(); cerr != nil && fnerr == nil {
			fmt.Fprintf(os.Stderr, "error closing resources file: %s\n", cerr.Error())
			fnerr = cerr
		}
	}()
	_, err = resfile.Write(resdata)
	if err != nil {
		fnerr = fmt.Errorf("failed to write resources file: %w", err)
		return
	}
	fmt.Printf("IDs for any newly created resources are stored in %s. Use the teardown command to clean them up.\n", resourcesFile)
}

func doTeardown(c *ocicloud.Client, res *resources) error {
	if res.InstanceID == nil {
		return nil
	}
	fmt.Printf("terminating instance %s\n", *res.InstanceID)
	if err := c.TerminateInstance(context.Background(), *res.InstanceID); err != nil {
		return err
	}
	fmt.Printf("instance %s terminated\n", *res.InstanceID)
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
		fnerr = fmt.Errorf("failed to read resources file: %w", err)
		return
	}
	res := &resources{}
	if err := json.Unmarshal(resdata, res); err != nil {
		fnerr = fmt.Errorf("failed to unmarshal resources data: %w", err)
		return
	}

	client, err := ocicloud.NewClient(res.Region)
	if err != nil {
		fnerr = err
		return
	}
	fnerr = doTeardown(client, res)
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
	ip := res.PublicIP
	if ip == "" {
		return fmt.Errorf("no public IP available for SSH")
	}
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

	region, fnerr := flags.GetString("region")
	if fnerr != nil {
		return
	}
	client, fnerr := ocicloud.NewClient(region)
	if fnerr != nil {
		return
	}

	res := &resources{Region: region}
	defer func() {
		if tderr := doTeardown(client, res); tderr != nil {
			fmt.Fprintf(os.Stderr, "teardown(): %s\n", tderr.Error())
		}
	}()

	fnerr = doSetup(client, flags, res)
	if fnerr != nil {
		return
	}
	fnerr = doRunExec(command, flags, res)
}

func flagDebug(flags *pflag.FlagSet) bool {
	debug, err := flags.GetBool("debug")
	if err != nil {
		return false
	}
	return debug
}

func addBootFlags(cmd *cobra.Command) {
	cmd.Flags().String("region", "", "target region")
	cmd.Flags().String("compartment-id", "", "compartment for the instance")
	cmd.Flags().String("image", "", "custom image OCID to boot")
	cmd.Flags().String("arch", "", "arch (x86_64 or aarch64)")
	cmd.Flags().String("subnet-id", "", "subnet to launch the instance in (must allow public IPs)")
	cmd.Flags().String("availability-domain", "", "availability domain for the instance")
	cmd.Flags().String("shape", "", "instance shape (optional; default from --arch)")
	cmd.Flags().String("shape-config", "", `flex shape JSON, e.g. {"ocpus": 1, "memoryInGBs": 4}`)
	cmd.Flags().String("ssh-authorized-keys-file", "", "path to SSH public key file (required for setup and run)")
}

func setupCLI() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:                   "boot-oci",
		Long:                  "boot a custom image on Oracle Cloud Infrastructure",
		DisableFlagsInUseLine: true,
	}

	rootFlags := rootCmd.PersistentFlags()
	rootFlags.Bool("debug", false, "print launch parameters before calling OCI")

	setupCmd := &cobra.Command{
		Use:                   "setup [--resourcefile <filename>]",
		Short:                 "boot an image and save the created resource IDs to a file for later teardown",
		Args:                  cobra.NoArgs,
		Run:                   setup,
		DisableFlagsInUseLine: true,
	}
	addBootFlags(setupCmd)
	setupCmd.Flags().StringP("resourcefile", "r", "resources.json", "path to store the resource IDs")
	for _, flag := range []string{"region", "compartment-id", "image", "arch", "subnet-id", "availability-domain", "ssh-authorized-keys-file"} {
		exitCheck(setupCmd.MarkFlagRequired(flag))
	}
	rootCmd.AddCommand(setupCmd)

	teardownCmd := &cobra.Command{
		Use:   "teardown [--resourcefile <filename>]",
		Short: "tear down resources from a previous setup",
		Args:  cobra.NoArgs,
		Run:   teardown,
	}
	teardownCmd.Flags().StringP("resourcefile", "r", "resources.json", "path containing the resource IDs")
	rootCmd.AddCommand(teardownCmd)

	runCmd := &cobra.Command{
		Use:   "run <executable>...",
		Short: "boot an image, upload an executable, and run it over SSH",
		Args:  cobra.MinimumNArgs(1),
		Run:   runExec,
	}
	addBootFlags(runCmd)
	runCmd.Flags().String("username", "", "SSH user on the instance")
	runCmd.Flags().String("ssh-privkey", "", "path to SSH private key")
	for _, flag := range []string{"region", "compartment-id", "image", "arch", "subnet-id", "availability-domain", "username", "ssh-privkey", "ssh-authorized-keys-file"} {
		exitCheck(runCmd.MarkFlagRequired(flag))
	}
	rootCmd.AddCommand(runCmd)

	return rootCmd
}

func main() {
	cmd := setupCLI()
	exitCheck(cmd.Execute())
}
