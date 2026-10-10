package oci

import "fmt"

// LogLaunchRequest prints resolved launch inputs before calling the OCI API.
func LogLaunchRequest(_ *Client, opts LaunchOptions) {
	fmt.Printf("LaunchInstance compartment=%s\n", opts.CompartmentID)
	fmt.Printf("LaunchInstance subnet=%s\n", opts.SubnetID)
	fmt.Printf("LaunchInstance availabilityDomain=%s\n", opts.AvailabilityDomain)
	fmt.Printf("LaunchInstance image=%s\n", opts.ImageID)
	fmt.Printf("LaunchInstance shape=%s\n", opts.Shape)
	if opts.ShapeConfig != nil {
		fmt.Printf("LaunchInstance shapeConfig ocpus=%s memoryInGBs=%s\n",
			formatFloat32(opts.ShapeConfig.Ocpus), formatFloat32(opts.ShapeConfig.MemoryInGBs))
	}
}

func formatFloat32(v *float32) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%g", *v)
}
