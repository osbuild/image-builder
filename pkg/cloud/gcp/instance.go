package gcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/google/uuid"
	"google.golang.org/api/option"

	"github.com/osbuild/image-builder/internal/common"
)

// BootInstance records cloud resources created for a one-off boot test.
type BootInstance struct {
	Project      string `json:"project"`
	Zone         string `json:"zone"`
	Name         string `json:"name"`
	FirewallName string `json:"firewall,omitempty"`
	Network      string `json:"network,omitempty"`
	PublicIP     string `json:"public-ip,omitempty"`
}

// BootInstanceOptions configures LaunchBootInstance.
type BootInstanceOptions struct {
	ImageName    string
	MachineType  string
	SSHKeys      string
	Network      string
	Subnetwork   string
	InstanceName string
}

// DefaultMachineType returns a modest test machine type for the architecture.
func DefaultMachineType(arch string) (string, error) {
	switch arch {
	case "x86_64":
		return "e2-medium", nil
	case "aarch64":
		return "t2a-standard-1", nil
	default:
		return "", fmt.Errorf("DefaultMachineType(): unknown architecture %q", arch)
	}
}

// LaunchBootInstance creates a firewall rule, boots a VM, and waits for a public IP.
func (g *GCP) LaunchBootInstance(ctx context.Context, zone string, opts BootInstanceOptions) (*BootInstance, error) {
	project := g.GetProjectID()
	if project == "" {
		return nil, fmt.Errorf("LaunchBootInstance: project ID is empty")
	}
	if opts.ImageName == "" {
		return nil, fmt.Errorf("LaunchBootInstance: image name is required")
	}

	name := opts.InstanceName
	if name == "" {
		name = fmt.Sprintf("image-boot-test-%s", uuid.New().String())
	}
	tag := fmt.Sprintf("image-boot-test-%s", uuid.New().String())
	fwName := fmt.Sprintf("image-boot-test-fw-%s", uuid.New().String())

	networkURL := opts.Network
	if networkURL == "" {
		networkURL = fmt.Sprintf("projects/%s/global/networks/default", project)
	}

	inst := &BootInstance{
		Project:      project,
		Zone:         zone,
		Name:         name,
		FirewallName: fwName,
		Network:      networkURL,
	}

	if err := g.insertBootFirewall(ctx, project, fwName, networkURL, tag); err != nil {
		return nil, err
	}

	if err := g.insertBootInstance(ctx, zone, inst, opts, tag); err != nil {
		_ = g.deleteBootFirewall(ctx, project, fwName)
		return nil, err
	}

	ip, err := g.instancePublicIP(ctx, zone, name)
	if err != nil {
		_ = g.DeleteBootInstance(ctx, inst)
		return nil, err
	}
	inst.PublicIP = ip
	return inst, nil
}

// DeleteBootInstance removes the VM and any boot-test firewall rule.
func (g *GCP) DeleteBootInstance(ctx context.Context, inst *BootInstance) error {
	if inst == nil {
		return nil
	}
	if inst.Name != "" && inst.Zone != "" {
		if err := g.deleteBootInstance(ctx, inst.Zone, inst.Name); err != nil {
			return err
		}
	}
	if inst.FirewallName != "" {
		if err := g.deleteBootFirewall(ctx, inst.Project, inst.FirewallName); err != nil {
			return err
		}
	}
	return nil
}

func (g *GCP) insertBootFirewall(ctx context.Context, project, name, networkURL, targetTag string) error {
	client, err := compute.NewFirewallsRESTClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return fmt.Errorf("insertBootFirewall: %w", err)
	}
	defer client.Close()

	req := &computepb.InsertFirewallRequest{
		Project: project,
		FirewallResource: &computepb.Firewall{
			Name:    &name,
			Network: &networkURL,
			TargetTags: []string{targetTag},
			Allowed: []*computepb.Allowed{
				{
					IPProtocol: common.ToPtr("tcp"),
					Ports:      []string{"22"},
				},
			},
			SourceRanges: []string{"0.0.0.0/0"},
			Description:  common.ToPtr("ephemeral SSH for image-builder boot-gcp tests"),
		},
	}
	op, err := client.Insert(ctx, req)
	if err != nil {
		return fmt.Errorf("insertBootFirewall Insert: %w", err)
	}
	return g.waitGlobalOperation(ctx, project, op.Proto().GetName())
}

func (g *GCP) deleteBootFirewall(ctx context.Context, project, name string) error {
	client, err := compute.NewFirewallsRESTClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return fmt.Errorf("deleteBootFirewall: %w", err)
	}
	defer client.Close()

	op, err := client.Delete(ctx, &computepb.DeleteFirewallRequest{
		Project:  project,
		Firewall: name,
	})
	if err != nil {
		return fmt.Errorf("deleteBootFirewall Delete: %w", err)
	}
	return g.waitGlobalOperation(ctx, project, op.Proto().GetName())
}

func (g *GCP) insertBootInstance(ctx context.Context, zone string, inst *BootInstance, opts BootInstanceOptions, tag string) error {
	client, err := compute.NewInstancesRESTClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return fmt.Errorf("insertBootInstance: %w", err)
	}
	defer client.Close()

	project := inst.Project
	sourceImage := fmt.Sprintf("projects/%s/global/images/%s", project, opts.ImageName)
	machineType := fmt.Sprintf("zones/%s/machineTypes/%s", zone, opts.MachineType)

	nic := &computepb.NetworkInterface{
		AccessConfigs: []*computepb.AccessConfig{
			{
				Name:        common.ToPtr("External NAT"),
				Type:        common.ToPtr("ONE_TO_ONE_NAT"),
				NetworkTier: common.ToPtr("PREMIUM"),
			},
		},
	}
	if opts.Subnetwork != "" {
		region, err := zoneRegion(zone)
		if err != nil {
			return err
		}
		subnetURL := opts.Subnetwork
		if !strings.Contains(subnetURL, "/") {
			subnetURL = fmt.Sprintf("projects/%s/regions/%s/subnetworks/%s", project, region, opts.Subnetwork)
		}
		nic.Subnetwork = &subnetURL
	} else if opts.Network != "" {
		nic.Network = &opts.Network
	}

	metadata := &computepb.Metadata{}
	if opts.SSHKeys != "" {
		metadata.Items = []*computepb.Items{
			{Key: common.ToPtr("ssh-keys"), Value: common.ToPtr(opts.SSHKeys)},
		}
	}

	req := &computepb.InsertInstanceRequest{
		Project: project,
		Zone:    zone,
		InstanceResource: &computepb.Instance{
			Name:        &inst.Name,
			MachineType: &machineType,
			Disks: []*computepb.AttachedDisk{
				{
					AutoDelete: common.ToPtr(true),
					Boot:       common.ToPtr(true),
					InitializeParams: &computepb.AttachedDiskInitializeParams{
						SourceImage: &sourceImage,
					},
				},
			},
			NetworkInterfaces: []*computepb.NetworkInterface{nic},
			Metadata:          metadata,
			Tags: &computepb.Tags{
				Items: []string{tag},
			},
		},
	}

	op, err := client.Insert(ctx, req)
	if err != nil {
		return fmt.Errorf("insertBootInstance Insert: %w", err)
	}
	return g.waitZoneOperation(ctx, project, zone, op.Proto().GetName())
}

func (g *GCP) deleteBootInstance(ctx context.Context, zone, name string) error {
	client, err := compute.NewInstancesRESTClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return fmt.Errorf("deleteBootInstance: %w", err)
	}
	defer client.Close()

	op, err := client.Delete(ctx, &computepb.DeleteInstanceRequest{
		Project:  g.GetProjectID(),
		Zone:     zone,
		Instance: name,
	})
	if err != nil {
		return fmt.Errorf("deleteBootInstance Delete: %w", err)
	}
	return g.waitZoneOperation(ctx, g.GetProjectID(), zone, op.Proto().GetName())
}

func (g *GCP) instancePublicIP(ctx context.Context, zone, name string) (string, error) {
	client, err := compute.NewInstancesRESTClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return "", fmt.Errorf("instancePublicIP: %w", err)
	}
	defer client.Close()

	deadline := time.Now().Add(30 * time.Minute)
	for time.Now().Before(deadline) {
		resp, err := client.Get(ctx, &computepb.GetInstanceRequest{
			Project:  g.GetProjectID(),
			Zone:     zone,
			Instance: name,
		})
		if err != nil {
			return "", fmt.Errorf("instancePublicIP Get: %w", err)
		}
		if resp.GetStatus() != computepb.Instance_RUNNING.String() {
			if err := sleepWithContext(ctx, 5*time.Second); err != nil {
				return "", fmt.Errorf("instancePublicIP: %w", err)
			}
			continue
		}
		for _, nic := range resp.GetNetworkInterfaces() {
			for _, ac := range nic.GetAccessConfigs() {
				if ip := ac.GetNatIP(); ip != "" {
					return ip, nil
				}
			}
		}
		return "", fmt.Errorf("instance %s has no public IP", name)
	}
	return "", fmt.Errorf("instance %s did not reach RUNNING within timeout", name)
}

// FormatSSHKeysMetadata builds the GCE ssh-keys metadata value from a pubkey file body.
func FormatSSHKeysMetadata(username, keyFileBody string) (string, error) {
	if strings.TrimSpace(keyFileBody) == "" {
		return "", fmt.Errorf("FormatSSHKeysMetadata: empty SSH public key")
	}
	if username == "" {
		return "", fmt.Errorf("FormatSSHKeysMetadata: username is required")
	}
	lines := strings.Split(strings.TrimSpace(keyFileBody), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, ":") {
			out = append(out, line)
			continue
		}
		out = append(out, fmt.Sprintf("%s:%s", username, line))
	}
	if len(out) == 0 {
		return "", fmt.Errorf("FormatSSHKeysMetadata: no SSH public keys found")
	}
	return strings.Join(out, "\n"), nil
}

// sleepWithContext waits unless the caller cancels the context first.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
