package gcp

import (
	"testing"

	"cloud.google.com/go/compute/apiv1/computepb"
)

func TestGuestOsFeaturesByDistroFedora(t *testing.T) {
	features := GuestOsFeaturesByDistro("fedora")
	if features == nil {
		t.Fatal("GuestOsFeaturesByDistro(\"fedora\") = nil, want Fedora features")
	}

	want := []string{
		computepb.GuestOsFeature_UEFI_COMPATIBLE.String(),
		computepb.GuestOsFeature_VIRTIO_SCSI_MULTIQUEUE.String(),
		computepb.GuestOsFeature_SEV_CAPABLE.String(),
		computepb.GuestOsFeature_GVNIC.String(),
	}
	if len(features) != len(want) {
		t.Fatalf("len(features) = %d, want %d", len(features), len(want))
	}
	for idx, feature := range features {
		if feature.GetType() != want[idx] {
			t.Errorf("features[%d] = %q, want %q", idx, feature.GetType(), want[idx])
		}
	}
}
