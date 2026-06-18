package osbuild

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/osbuild/image-builder/internal/common"
)

func TestNewChronyStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.chrony",
		Options: &ChronyStageOptions{},
	}
	actualStage := NewChronyStage(&ChronyStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestChronyConfigServerValidation(t *testing.T) {
	tests := map[string]struct {
		server ChronyConfigServer
		expErr string
	}{
		"valid-server": {
			server: ChronyConfigServer{
				Hostname: "time.example.org",
				Minpoll:  common.ToPtr(4),
				Maxpoll:  common.ToPtr(10),
			},
			expErr: "",
		},
		"no-hostname": {
			server: ChronyConfigServer{
				Minpoll: common.ToPtr(4),
				Maxpoll: common.ToPtr(10),
			},
			expErr: "org.osbuild.chrony: server hostname is required",
		},
		"bad-minpoll": {
			server: ChronyConfigServer{
				Hostname: "time.example.org",
				Minpoll:  common.ToPtr(-7),
				Maxpoll:  common.ToPtr(10),
			},
			expErr: "org.osbuild.chrony: invalid server minpoll: must be in the range [-6, 24]",
		},
		"bad-maxpoll": {
			server: ChronyConfigServer{
				Hostname: "time.example.org",
				Minpoll:  common.ToPtr(4),
				Maxpoll:  common.ToPtr(25),
			},
			expErr: "org.osbuild.chrony: invalid server maxpoll: must be in the range [-6, 24]",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			err := tc.server.validate()
			if tc.expErr == "" {
				assert.NoError(err)
			} else {
				assert.EqualError(err, tc.expErr)
			}
		})
	}
}

func TestChronyConfigRefclockValidation(t *testing.T) {
	tests := map[string]struct {
		refclock ChronyConfigRefclock
		expErr   string
	}{
		"valid-pps": {
			refclock: ChronyConfigRefclock{
				Driver: ChronyDriverPPS{Name: "PPS", Device: "/dev/pps0"},
			},
			expErr: "",
		},
		"invalid-pps-name": {
			refclock: ChronyConfigRefclock{
				Driver: ChronyDriverPPS{Name: "PPSX", Device: "/dev/pps0"},
			},
			expErr: "org.osbuild.chrony: invalid PPS driver name \"PPSX\"",
		},
		"invalid-pps-device": {
			refclock: ChronyConfigRefclock{
				Driver: ChronyDriverPPS{Name: "PPS", Device: "../pps0"},
			},
			expErr: fmt.Sprintf("org.osbuild.chrony: invalid PPS device path: \"../pps0\" matches invalid path regular expression %q", invalidPathRegex),
		},
		"valid-shm": {
			refclock: ChronyConfigRefclock{
				Driver: ChronyDriverSHM{Name: "SHM", Segment: 0},
			},
			expErr: "",
		},
		"invalid-shm-name": {
			refclock: ChronyConfigRefclock{
				Driver: ChronyDriverSHM{Name: "invalid", Segment: 0},
			},
			expErr: "org.osbuild.chrony: invalid SHM driver name \"invalid\"",
		},
		"invalid-shm-perm": {
			refclock: ChronyConfigRefclock{
				Driver: ChronyDriverSHM{Name: "SHM", Segment: 0, Perm: common.ToPtr("0888")},
			},
			expErr: fmt.Sprintf("org.osbuild.chrony: invalid SHM driver perm: \"0888\" doesn't match perm regular expression %q", chronyStagePermRegex),
		},
		"valid-sock": {
			refclock: ChronyConfigRefclock{
				Driver: ChronyDriverSOCK{Name: "SOCK", Path: "/var/run/chrony.sock"},
			},
			expErr: "",
		},
		"invalid-sock": {
			refclock: ChronyConfigRefclock{
				Driver: ChronyDriverSOCK{Name: "socksock", Path: "/var/run/chrony.sock"},
			},
			expErr: "org.osbuild.chrony: invalid SOCK driver name \"socksock\"",
		},
		"invalid-sock-path": {
			refclock: ChronyConfigRefclock{
				Driver: ChronyDriverSOCK{Name: "SOCK", Path: "../var/run/chrony.sock"},
			},
			expErr: fmt.Sprintf("org.osbuild.chrony: invalid SOCK socket path: \"../var/run/chrony.sock\" matches invalid path regular expression %q", invalidPathRegex),
		},
		"valid-phc": {
			refclock: ChronyConfigRefclock{
				Driver: ChronyDriverPHC{Name: "PHC", Path: "/dev/ptp0"},
			},
			expErr: "",
		},
		"invalid-phc": {
			refclock: ChronyConfigRefclock{
				Driver: ChronyDriverPHC{Name: "notPHC", Path: "/dev/ptp0"},
			},
			expErr: "org.osbuild.chrony: invalid PHC driver name \"notPHC\"",
		},
		"invalid-phc-path": {
			refclock: ChronyConfigRefclock{
				Driver: ChronyDriverPHC{Name: "PHC", Path: "../dev/ptp0"},
			},
			expErr: fmt.Sprintf("org.osbuild.chrony: invalid PHC path: \"../dev/ptp0\" matches invalid path regular expression %q", invalidPathRegex),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			err := tc.refclock.validate()
			if tc.expErr == "" {
				assert.NoError(err)
			} else {
				assert.EqualError(err, tc.expErr)
			}
		})
	}
}

func TestChronyConfigRefclockConstructors(t *testing.T) {
	assert := assert.New(t)

	// ensure each constructor sets the correct name and parameter
	assert.Equal(NewChronyDriverPPS("/dev/pps10020"), ChronyDriverPPS{Name: "PPS", Device: "/dev/pps10020"})
	assert.Equal(NewChronyDriverSHM(290), ChronyDriverSHM{Name: "SHM", Segment: 290})
	assert.Equal(NewChronyDriverSOCK("/run/chrony/a-single-wool-sock"), ChronyDriverSOCK{Name: "SOCK", Path: "/run/chrony/a-single-wool-sock"})
	assert.Equal(NewChronyDriverPHC("/dev/thingie"), ChronyDriverPHC{Name: "PHC", Path: "/dev/thingie"})
}

func TestChronyStageOptionsValidation(t *testing.T) {
	tests := map[string]struct {
		options ChronyStageOptions
		expErr  string
	}{
		"valid": {
			options: ChronyStageOptions{
				Servers: []ChronyConfigServer{
					{Hostname: "time.google.com", Minpoll: common.ToPtr(4), Maxpoll: common.ToPtr(10)},
				},
				Refclocks: []ChronyConfigRefclock{
					{Driver: ChronyDriverPPS{Name: "PPS", Device: "/dev/pps0"}},
				},
				LeapsecTz: common.ToPtr("right/UTC"),
			},
			expErr: "",
		},
		"invalid-server": {
			options: ChronyStageOptions{
				Servers: []ChronyConfigServer{
					{Hostname: "", Minpoll: common.ToPtr(4), Maxpoll: common.ToPtr(10)},
				},
				Refclocks: []ChronyConfigRefclock{
					{Driver: ChronyDriverPPS{Name: "PPS", Device: "/dev/pps0"}},
				},
				LeapsecTz: common.ToPtr("right/UTC"),
			},
			expErr: "org.osbuild.chrony: server hostname is required",
		},
		"inavlid-refclock": {
			options: ChronyStageOptions{
				Servers: []ChronyConfigServer{
					{Hostname: "time.google.com", Minpoll: common.ToPtr(4), Maxpoll: common.ToPtr(10)},
				},
				Refclocks: []ChronyConfigRefclock{
					{Driver: ChronyDriverPPS{Name: "PPSX", Device: "/dev/pps0"}},
				},
				LeapsecTz: common.ToPtr("right/UTC"),
			},
			expErr: "org.osbuild.chrony: invalid PPS driver name \"PPSX\"",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			err := tc.options.validate()
			if tc.expErr == "" {
				assert.NoError(err)
			} else {
				assert.EqualError(err, tc.expErr)
			}
		})
	}
}

func TestChronyStageUnmarshal(t *testing.T) {
	inputYAML := `
time_synchronization:
  refclocks:
   - driver:
       name: "PPS"
       device: "/dev/some-device"
     poll: 3
     dpoll: -2
     offset: 0.0
   - driver:
       name: "SHM"
       segment: 123
   - driver:
       name: "SOCK"
       path: "/some/path"
   - driver:
       name: "PHC"
       path: "/dev/ptp_hyperv"
`
	var opts struct {
		TS *ChronyStageOptions `json:"time_synchronization" yaml:"time_synchronization"`
	}
	err := yaml.Unmarshal([]byte(inputYAML), &opts)
	require.NoError(t, err)
	assert.Equal(t, &ChronyStageOptions{
		Refclocks: []ChronyConfigRefclock{
			{
				Driver: &ChronyDriverPPS{
					Name:   "PPS",
					Device: "/dev/some-device",
				},
				Poll:   common.ToPtr(3),
				Dpoll:  common.ToPtr(-2),
				Offset: common.ToPtr(0.0),
			},
			{
				Driver: &ChronyDriverSHM{
					Name:    "SHM",
					Segment: 123,
				},
			},
			{
				Driver: &ChronyDriverSOCK{
					Name: "SOCK",
					Path: "/some/path",
				},
			},
			{
				Driver: &ChronyDriverPHC{
					Name: "PHC",
					Path: "/dev/ptp_hyperv",
				},
			},
		},
	}, opts.TS)
}
