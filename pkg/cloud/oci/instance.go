package oci

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/oracle/oci-go-sdk/v54/common"
	"github.com/oracle/oci-go-sdk/v54/core"
)

// LaunchOptions configures a one-off test instance.
type LaunchOptions struct {
	CompartmentID      string
	SubnetID           string
	ImageID            string
	Shape              string
	ShapeConfig        *core.LaunchInstanceShapeConfigDetails
	AvailabilityDomain string
	SSHAuthorizedKeys  string
	DisplayName        string
}

// DefaultShape picks a modest test shape for the given architecture.
func DefaultShape(arch string) (string, *core.LaunchInstanceShapeConfigDetails, error) {
	switch arch {
	case "x86_64":
		return "VM.Standard.E4.Flex", &core.LaunchInstanceShapeConfigDetails{
			Ocpus:       common.Float32(1),
			MemoryInGBs: common.Float32(4),
		}, nil
	case "aarch64":
		return "VM.Standard.A1.Flex", &core.LaunchInstanceShapeConfigDetails{
			Ocpus:       common.Float32(1),
			MemoryInGBs: common.Float32(6),
		}, nil
	default:
		return "", nil, fmt.Errorf("DefaultShape(): unknown architecture %q", arch)
	}
}

// LaunchInstance boots a VM from a custom image and waits until it is running.
func (c *Client) LaunchInstance(ctx context.Context, opts LaunchOptions) (string, error) {
	if opts.AvailabilityDomain == "" {
		return "", fmt.Errorf("LaunchInstance: availability domain is required")
	}

	name := opts.DisplayName
	if name == "" {
		name = fmt.Sprintf("image-boot-test-%s", uuid.New().String())
	}

	metadata := map[string]string{}
	if opts.SSHAuthorizedKeys != "" {
		metadata["ssh_authorized_keys"] = opts.SSHAuthorizedKeys
	}

	details := core.LaunchInstanceDetails{
		AvailabilityDomain: common.String(opts.AvailabilityDomain),
		CompartmentId:      common.String(opts.CompartmentID),
		DisplayName:        common.String(name),
		Shape:              common.String(opts.Shape),
		ShapeConfig:        opts.ShapeConfig,
		Metadata:           metadata,
		SourceDetails: core.InstanceSourceViaImageDetails{
			ImageId: common.String(opts.ImageID),
		},
		CreateVnicDetails: &core.CreateVnicDetails{
			SubnetId:       common.String(opts.SubnetID),
			AssignPublicIp: common.Bool(true),
			DisplayName:    common.String(name + "-vnic"),
		},
	}

	resp, err := c.compute.LaunchInstance(ctx, core.LaunchInstanceRequest{LaunchInstanceDetails: details})
	if err != nil {
		return "", fmt.Errorf("LaunchInstance: %w", err)
	}
	instanceID := *resp.Instance.Id

	if err := c.waitInstanceRunning(ctx, instanceID); err != nil {
		_ = c.TerminateInstance(ctx, instanceID)
		return "", err
	}

	return instanceID, nil
}

func (c *Client) waitInstanceRunning(ctx context.Context, instanceID string) error {
	deadline := time.Now().Add(30 * time.Minute)
	for time.Now().Before(deadline) {
		resp, err := c.compute.GetInstance(ctx, core.GetInstanceRequest{InstanceId: common.String(instanceID)})
		if err != nil {
			return fmt.Errorf("GetInstance: %w", err)
		}
		switch resp.Instance.LifecycleState {
		case core.InstanceLifecycleStateRunning:
			return nil
		case core.InstanceLifecycleStateTerminated, core.InstanceLifecycleStateTerminating:
			return fmt.Errorf("instance %s entered state %s while waiting to run", instanceID, resp.Instance.LifecycleState)
		}
		if err := sleepWithContext(ctx, 10*time.Second); err != nil {
			return err
		}
	}
	return fmt.Errorf("timed out waiting for instance %s to run", instanceID)
}

// InstancePublicIP returns the first public IP attached to the instance.
func (c *Client) InstancePublicIP(ctx context.Context, compartmentID, instanceID string) (string, error) {
	listResp, err := c.compute.ListVnicAttachments(ctx, core.ListVnicAttachmentsRequest{
		CompartmentId: common.String(compartmentID),
		InstanceId:    common.String(instanceID),
	})
	if err != nil {
		return "", fmt.Errorf("ListVnicAttachments: %w", err)
	}
	if len(listResp.Items) == 0 {
		return "", fmt.Errorf("instance %s has no VNIC attachments", instanceID)
	}
	vnicID := listResp.Items[0].VnicId
	if vnicID == nil {
		return "", fmt.Errorf("instance %s VNIC attachment has no vnic id", instanceID)
	}

	vnicResp, err := c.virtualNetwork.GetVnic(ctx, core.GetVnicRequest{VnicId: vnicID})
	if err != nil {
		return "", fmt.Errorf("GetVnic: %w", err)
	}
	if vnicResp.Vnic.PublicIp == nil || *vnicResp.Vnic.PublicIp == "" {
		return "", fmt.Errorf("instance %s has no public IP on subnet (use a public subnet)", instanceID)
	}
	return *vnicResp.Vnic.PublicIp, nil
}

// TerminateInstance deletes the test VM and waits until it is gone.
func (c *Client) TerminateInstance(ctx context.Context, instanceID string) error {
	_, err := c.compute.TerminateInstance(ctx, core.TerminateInstanceRequest{
		InstanceId: common.String(instanceID),
	})
	if err != nil {
		return fmt.Errorf("TerminateInstance: %w", err)
	}

	deadline := time.Now().Add(30 * time.Minute)
	for time.Now().Before(deadline) {
		resp, err := c.compute.GetInstance(ctx, core.GetInstanceRequest{InstanceId: common.String(instanceID)})
		if err != nil {
			return fmt.Errorf("GetInstance: %w", err)
		}
		if resp.Instance.LifecycleState == core.InstanceLifecycleStateTerminated {
			return nil
		}
		if err := sleepWithContext(ctx, 10*time.Second); err != nil {
			return err
		}
	}
	return fmt.Errorf("timed out waiting for instance %s to terminate", instanceID)
}

// sleepWithContext waits unless the caller cancels the stop signal first.
func sleepWithContext(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
