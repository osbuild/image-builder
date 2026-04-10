package check_test

import (
	"os"
	"strings"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	check "github.com/osbuild/images/cmd/check-host-config/check"
	"github.com/osbuild/images/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenSCAPCheck(t *testing.T) {
	test.MockGlobal(t, &check.Exists, func(name string) bool {
		return name == "results.xml"
	})

	// Mock XML file content with score 85.0 and no high severity failures (XCCDF 1.2 format)
	xmlContent := `<?xml version="1.0"?>
<Benchmark xmlns="http://checklists.nist.gov/xccdf/1.2">
	<TestResult>
		<score>85.0</score>
		<rule-result severity="high" idref="rule1">
			<result>pass</result>
		</rule-result>
		<rule-result severity="medium" idref="rule2">
			<result>fail</result>
		</rule-result>
	</TestResult>
</Benchmark>`

	test.MockGlobal(t, &check.ParseOSRelease, func(osReleasePath string) (*check.OSRelease, error) {
		return &check.OSRelease{
			ID:           "rhel",
			VersionID:    "9.0",
			Version:      "9.0 (Plow)",
			MajorVersion: 9,
		}, nil
	})

	test.MockGlobal(t, &check.ReadFile, func(filename string) ([]byte, error) {
		if filename == "results.xml" {
			return []byte(xmlContent), nil
		}
		return os.ReadFile(filename)
	})

	test.MockGlobal(t, &check.Exec, func(name string, arg ...string) ([]byte, []byte, int, error) {
		cmd := joinArgs(name, arg...)
		if strings.HasPrefix(cmd, "sudo oscap") {
			return []byte(""), nil, 0, nil
		}
		if strings.HasPrefix(cmd, "sudo chown") {
			return []byte(""), nil, 0, nil
		}
		return nil, nil, 0, nil
	})

	chk, found := check.FindCheckByName("oscap")
	require.True(t, found, "OpenSCAP Check not found")
	config := buildConfig(&blueprint.Customizations{
		OpenSCAP: &blueprint.OpenSCAPCustomization{
			ProfileID:  "xccdf_org.ssgproject.content_profile_ospp",
			DataStream: "/usr/share/xml/scap/ssg/content/ssg-rhel9-ds.xml",
		},
	})

	require.NoError(t, chk.Func(chk.Meta, config))
}

func TestOpenSCAPCheckSkip(t *testing.T) {
	chk, found := check.FindCheckByName("oscap")
	require.True(t, found, "OpenSCAP Check not found")
	config := buildConfig(&blueprint.Customizations{
		OpenSCAP: nil,
	})

	err := chk.Func(chk.Meta, config)
	require.Error(t, err)
	assert.True(t, check.IsSkip(err))
}

func TestOpenSCAPCheckSkipIncomplete(t *testing.T) {
	chk, found := check.FindCheckByName("oscap")
	require.True(t, found, "OpenSCAP Check not found")
	config := buildConfig(&blueprint.Customizations{
		OpenSCAP: &blueprint.OpenSCAPCustomization{
			ProfileID:  "",
			DataStream: "/usr/share/xml/scap/ssg/content/ssg-rhel9-ds.xml",
		},
	})

	err := chk.Func(chk.Meta, config)
	require.Error(t, err)
	assert.True(t, check.IsSkip(err))
}

func TestOpenSCAPCheckFailNoResults(t *testing.T) {
	test.MockGlobal(t, &check.Exists, func(name string) bool {
		// results.xml does not exist
		return false
	})

	test.MockGlobal(t, &check.Exec, func(name string, arg ...string) ([]byte, []byte, int, error) {
		cmd := joinArgs(name, arg...)
		if strings.HasPrefix(cmd, "sudo oscap") {
			return []byte("oscap error"), nil, 0, nil
		}
		if strings.HasPrefix(cmd, "sudo chown") {
			return []byte(""), nil, 0, nil
		}
		return nil, nil, 0, nil
	})

	chk, found := check.FindCheckByName("oscap")
	require.True(t, found, "OpenSCAP Check not found")
	config := buildConfig(&blueprint.Customizations{
		OpenSCAP: &blueprint.OpenSCAPCustomization{
			ProfileID:  "xccdf_org.ssgproject.content_profile_ospp",
			DataStream: "/usr/share/xml/scap/ssg/content/ssg-rhel9-ds.xml",
		},
	})

	err := chk.Func(chk.Meta, config)
	require.Error(t, err)
	assert.True(t, check.IsFail(err))
}

func TestOpenSCAPCheckFailLowScore(t *testing.T) {
	test.MockGlobal(t, &check.Exists, func(name string) bool {
		return name == "results.xml"
	})

	// Mock XML file content with low score (70.0, below baseline of 80.0) (XCCDF 1.2 format)
	xmlContent := `<?xml version="1.0"?>
<Benchmark xmlns="http://checklists.nist.gov/xccdf/1.2">
	<TestResult>
		<score>70.0</score>
		<rule-result severity="high" idref="rule1">
			<result>pass</result>
		</rule-result>
	</TestResult>
</Benchmark>`

	test.MockGlobal(t, &check.ParseOSRelease, func(osReleasePath string) (*check.OSRelease, error) {
		return &check.OSRelease{
			ID:           "rhel",
			VersionID:    "9.0",
			Version:      "9.0 (Plow)",
			MajorVersion: 9,
		}, nil
	})

	test.MockGlobal(t, &check.ReadFile, func(filename string) ([]byte, error) {
		if filename == "results.xml" {
			return []byte(xmlContent), nil
		}
		return os.ReadFile(filename)
	})

	test.MockGlobal(t, &check.Exec, func(name string, arg ...string) ([]byte, []byte, int, error) {
		cmd := joinArgs(name, arg...)
		if strings.HasPrefix(cmd, "sudo oscap") {
			return []byte(""), nil, 0, nil
		}
		if strings.HasPrefix(cmd, "sudo chown") {
			return []byte(""), nil, 0, nil
		}
		return nil, nil, 0, nil
	})

	chk, found := check.FindCheckByName("oscap")
	require.True(t, found, "OpenSCAP Check not found")
	config := buildConfig(&blueprint.Customizations{
		OpenSCAP: &blueprint.OpenSCAPCustomization{
			ProfileID:  "xccdf_org.ssgproject.content_profile_ospp",
			DataStream: "/usr/share/xml/scap/ssg/content/ssg-rhel9-ds.xml",
		},
	})

	err := chk.Func(chk.Meta, config)
	require.Error(t, err)
	assert.True(t, check.IsFail(err))
}

func TestOpenSCAPCheckFailHighSeverityRules(t *testing.T) {
	test.MockGlobal(t, &check.Exists, func(name string) bool {
		return name == "results.xml"
	})

	// Mock XML file content with good score but high severity failures (XCCDF 1.2 format)
	xmlContent := `<?xml version="1.0"?>
<Benchmark xmlns="http://checklists.nist.gov/xccdf/1.2">
	<TestResult>
		<score>85.0</score>
		<rule-result severity="high" idref="rule1">
			<result>fail</result>
		</rule-result>
		<rule-result severity="high" idref="rule2">
			<result>pass</result>
		</rule-result>
		<rule-result severity="high" idref="rule3">
			<result>fail</result>
		</rule-result>
	</TestResult>
</Benchmark>`

	test.MockGlobal(t, &check.ParseOSRelease, func(osReleasePath string) (*check.OSRelease, error) {
		return &check.OSRelease{
			ID:           "rhel",
			VersionID:    "9.0",
			Version:      "9.0 (Plow)",
			MajorVersion: 9,
		}, nil
	})

	test.MockGlobal(t, &check.ReadFile, func(filename string) ([]byte, error) {
		if filename == "results.xml" {
			return []byte(xmlContent), nil
		}
		return os.ReadFile(filename)
	})

	test.MockGlobal(t, &check.Exec, func(name string, arg ...string) ([]byte, []byte, int, error) {
		cmd := joinArgs(name, arg...)
		if strings.HasPrefix(cmd, "sudo oscap") {
			return []byte(""), nil, 0, nil
		}
		if strings.HasPrefix(cmd, "sudo chown") {
			return []byte(""), nil, 0, nil
		}
		return nil, nil, 0, nil
	})

	chk, found := check.FindCheckByName("oscap")
	require.True(t, found, "OpenSCAP Check not found")
	config := buildConfig(&blueprint.Customizations{
		OpenSCAP: &blueprint.OpenSCAPCustomization{
			ProfileID:  "xccdf_org.ssgproject.content_profile_ospp",
			DataStream: "/usr/share/xml/scap/ssg/content/ssg-rhel9-ds.xml",
		},
	})

	err := chk.Func(chk.Meta, config)
	require.Error(t, err)
	assert.True(t, check.IsFail(err))
}

func TestOpenSCAPCheckIgnoreHighSeverityRules(t *testing.T) {
	test.MockGlobal(t, &check.Exists, func(name string) bool {
		return name == "results.xml"
	})

	// Mock XML file content with good score and ignored high severity failure (XCCDF 1.2 format)
	// The ignored rule should not cause the check to fail
	xmlContent := `<?xml version="1.0"?>
<Benchmark xmlns="http://checklists.nist.gov/xccdf/1.2">
	<TestResult>
		<score>85.0</score>
		<rule-result severity="high" idref="xccdf_org.ssgproject.content_rule_ensure_redhat_gpgkey_installed">
			<result>fail</result>
		</rule-result>
		<rule-result severity="high" idref="rule2">
			<result>pass</result>
		</rule-result>
	</TestResult>
</Benchmark>`

	test.MockGlobal(t, &check.ParseOSRelease, func(osReleasePath string) (*check.OSRelease, error) {
		return &check.OSRelease{
			ID:           "rhel",
			VersionID:    "9.0",
			Version:      "9.0 (Plow)",
			MajorVersion: 9,
		}, nil
	})

	test.MockGlobal(t, &check.ReadFile, func(filename string) ([]byte, error) {
		if filename == "results.xml" {
			return []byte(xmlContent), nil
		}
		return os.ReadFile(filename)
	})

	test.MockGlobal(t, &check.Exec, func(name string, arg ...string) ([]byte, []byte, int, error) {
		cmd := joinArgs(name, arg...)
		if strings.HasPrefix(cmd, "sudo oscap") {
			return []byte(""), nil, 0, nil
		}
		if strings.HasPrefix(cmd, "sudo chown") {
			return []byte(""), nil, 0, nil
		}
		return nil, nil, 0, nil
	})

	chk, found := check.FindCheckByName("oscap")
	require.True(t, found, "OpenSCAP Check not found")
	config := buildConfig(&blueprint.Customizations{
		OpenSCAP: &blueprint.OpenSCAPCustomization{
			ProfileID:  "xccdf_org.ssgproject.content_profile_ospp",
			DataStream: "/usr/share/xml/scap/ssg/content/ssg-rhel9-ds.xml",
		},
	})

	require.NoError(t, chk.Func(chk.Meta, config))
}

func TestOpenSCAPCheckIgnoreAndFailHighSeverityRules(t *testing.T) {
	test.MockGlobal(t, &check.Exists, func(name string) bool {
		return name == "results.xml"
	})

	// Mock XML file content with good score, one ignored high severity failure and one non-ignored failure
	// The check should fail because of the non-ignored rule, but the ignored rule should not appear in the error
	xmlContent := `<?xml version="1.0"?>
<Benchmark xmlns="http://checklists.nist.gov/xccdf/1.2">
	<TestResult>
		<score>85.0</score>
		<rule-result severity="high" idref="xccdf_org.ssgproject.content_rule_ensure_redhat_gpgkey_installed">
			<result>fail</result>
		</rule-result>
		<rule-result severity="high" idref="rule_non_ignored">
			<result>fail</result>
		</rule-result>
	</TestResult>
</Benchmark>`

	test.MockGlobal(t, &check.ParseOSRelease, func(osReleasePath string) (*check.OSRelease, error) {
		return &check.OSRelease{
			ID:           "rhel",
			VersionID:    "9.0",
			Version:      "9.0 (Plow)",
			MajorVersion: 9,
		}, nil
	})

	test.MockGlobal(t, &check.ReadFile, func(filename string) ([]byte, error) {
		if filename == "results.xml" {
			return []byte(xmlContent), nil
		}
		return os.ReadFile(filename)
	})

	test.MockGlobal(t, &check.Exec, func(name string, arg ...string) ([]byte, []byte, int, error) {
		cmd := joinArgs(name, arg...)
		if strings.HasPrefix(cmd, "sudo oscap") {
			return []byte(""), nil, 0, nil
		}
		if strings.HasPrefix(cmd, "sudo chown") {
			return []byte(""), nil, 0, nil
		}
		return nil, nil, 0, nil
	})

	chk, found := check.FindCheckByName("oscap")
	require.True(t, found, "OpenSCAP Check not found")
	config := buildConfig(&blueprint.Customizations{
		OpenSCAP: &blueprint.OpenSCAPCustomization{
			ProfileID:  "xccdf_org.ssgproject.content_profile_ospp",
			DataStream: "/usr/share/xml/scap/ssg/content/ssg-rhel9-ds.xml",
		},
	})

	err := chk.Func(chk.Meta, config)
	require.Error(t, err)
	assert.True(t, check.IsFail(err))
}

func TestOpenSCAPCheckFailExtractScore(t *testing.T) {
	test.MockGlobal(t, &check.Exists, func(name string) bool {
		return name == "results.xml"
	})

	// Mock XML file content without score element (should fail parsing)
	xmlContent := `<?xml version="1.0"?>
<Benchmark xmlns="http://checklists.nist.gov/xccdf/1.2">
	<TestResult>
		<rule-result severity="high" idref="rule1">
			<result>pass</result>
		</rule-result>
	</TestResult>
</Benchmark>`

	test.MockGlobal(t, &check.ParseOSRelease, func(osReleasePath string) (*check.OSRelease, error) {
		return &check.OSRelease{
			ID:           "rhel",
			VersionID:    "9.0",
			Version:      "9.0 (Plow)",
			MajorVersion: 9,
		}, nil
	})

	test.MockGlobal(t, &check.ReadFile, func(filename string) ([]byte, error) {
		if filename == "results.xml" {
			return []byte(xmlContent), nil
		}
		return os.ReadFile(filename)
	})

	test.MockGlobal(t, &check.Exec, func(name string, arg ...string) ([]byte, []byte, int, error) {
		cmd := joinArgs(name, arg...)
		if strings.HasPrefix(cmd, "sudo oscap") {
			return []byte(""), nil, 0, nil
		}
		if strings.HasPrefix(cmd, "sudo chown") {
			return []byte(""), nil, 0, nil
		}
		return nil, nil, 0, nil
	})

	chk, found := check.FindCheckByName("oscap")
	require.True(t, found, "OpenSCAP Check not found")
	config := buildConfig(&blueprint.Customizations{
		OpenSCAP: &blueprint.OpenSCAPCustomization{
			ProfileID:  "xccdf_org.ssgproject.content_profile_ospp",
			DataStream: "/usr/share/xml/scap/ssg/content/ssg-rhel9-ds.xml",
		},
	})

	err := chk.Func(chk.Meta, config)
	require.Error(t, err)
	assert.True(t, check.IsFail(err))
}

func TestOpenSCAPCheckFailExtractRules(t *testing.T) {
	test.MockGlobal(t, &check.Exists, func(name string) bool {
		return name == "results.xml"
	})

	// Mock XML file content with invalid XML (should fail parsing)
	xmlContent := `<?xml version="1.0"?>
<Benchmark xmlns="http://checklists.nist.gov/xccdf/1.2">
	<TestResult>
		<score>85.0</score>
		<rule-result severity="high" idref="rule1">
			<result>fail</result>
		</rule-result>
	</TestResult>
</Benchmark>`

	test.MockGlobal(t, &check.ParseOSRelease, func(osReleasePath string) (*check.OSRelease, error) {
		return &check.OSRelease{
			ID:           "rhel",
			VersionID:    "9.0",
			Version:      "9.0 (Plow)",
			MajorVersion: 9,
		}, nil
	})

	test.MockGlobal(t, &check.ReadFile, func(filename string) ([]byte, error) {
		if filename == "results.xml" {
			return []byte(xmlContent), nil
		}
		return os.ReadFile(filename)
	})

	test.MockGlobal(t, &check.Exec, func(name string, arg ...string) ([]byte, []byte, int, error) {
		cmd := joinArgs(name, arg...)
		if strings.HasPrefix(cmd, "sudo oscap") {
			return []byte(""), nil, 0, nil
		}
		if strings.HasPrefix(cmd, "sudo chown") {
			return []byte(""), nil, 0, nil
		}
		return nil, nil, 0, nil
	})

	chk, found := check.FindCheckByName("oscap")
	require.True(t, found, "OpenSCAP Check not found")
	config := buildConfig(&blueprint.Customizations{
		OpenSCAP: &blueprint.OpenSCAPCustomization{
			ProfileID:  "xccdf_org.ssgproject.content_profile_ospp",
			DataStream: "/usr/share/xml/scap/ssg/content/ssg-rhel9-ds.xml",
		},
	})

	err := chk.Func(chk.Meta, config)
	require.Error(t, err)
	assert.True(t, check.IsFail(err))
}

func TestOpenSCAPCheckNullDatastreamRHEL(t *testing.T) {
	test.MockGlobal(t, &check.Exists, func(name string) bool {
		return name == "results.xml"
	})

	// Mock XML file content with score 85.0 and no high severity failures (XCCDF 1.2 format)
	xmlContent := `<?xml version="1.0"?>
<Benchmark xmlns="http://checklists.nist.gov/xccdf/1.2">
	<TestResult>
		<score>85.0</score>
		<rule-result severity="high" idref="rule1">
			<result>pass</result>
		</rule-result>
		<rule-result severity="medium" idref="rule2">
			<result>fail</result>
		</rule-result>
	</TestResult>
</Benchmark>`

	test.MockGlobal(t, &check.ParseOSRelease, func(osReleasePath string) (*check.OSRelease, error) {
		return &check.OSRelease{
			ID:           "rhel",
			VersionID:    "9.0",
			Version:      "9.0 (Plow)",
			MajorVersion: 9,
		}, nil
	})

	test.MockGlobal(t, &check.ReadFile, func(filename string) ([]byte, error) {
		if filename == "results.xml" {
			return []byte(xmlContent), nil
		}
		return os.ReadFile(filename)
	})

	test.MockGlobal(t, &check.Exec, func(name string, arg ...string) ([]byte, []byte, int, error) {
		cmd := joinArgs(name, arg...)
		if strings.HasPrefix(cmd, "sudo oscap") {
			return []byte(""), nil, 0, nil
		}
		if strings.HasPrefix(cmd, "sudo chown") {
			return []byte(""), nil, 0, nil
		}
		return nil, nil, 0, nil
	})

	chk, found := check.FindCheckByName("oscap")
	require.True(t, found, "OpenSCAP Check not found")
	config := buildConfig(&blueprint.Customizations{
		OpenSCAP: &blueprint.OpenSCAPCustomization{
			ProfileID:  "xccdf_org.ssgproject.content_profile_ospp",
			DataStream: "null", // null datastream should trigger fallback
		},
	})

	require.NoError(t, chk.Func(chk.Meta, config))
}

func TestOpenSCAPCheckSkipRHEL7(t *testing.T) {
	test.MockGlobal(t, &check.ParseOSRelease, func(osReleasePath string) (*check.OSRelease, error) {
		return &check.OSRelease{
			ID:           "rhel",
			VersionID:    "7.9",
			Version:      "7.9 (Maipo)",
			MajorVersion: 7,
		}, nil
	})

	chk, found := check.FindCheckByName("oscap")
	require.True(t, found, "OpenSCAP Check not found")
	config := buildConfig(&blueprint.Customizations{
		OpenSCAP: &blueprint.OpenSCAPCustomization{
			ProfileID:  "xccdf_org.ssgproject.content_profile_ospp",
			DataStream: "/usr/share/xml/scap/ssg/content/ssg-rhel7-ds.xml",
		},
	})

	err := chk.Func(chk.Meta, config)
	require.Error(t, err)
	assert.True(t, check.IsSkip(err))
	assert.Contains(t, err.Error(), "only XCCDF 1.2 is supported")
}

func TestOpenSCAPCheckFailNoTestResult(t *testing.T) {
	test.MockGlobal(t, &check.Exists, func(name string) bool {
		return name == "results.xml"
	})

	// Mock XML file content with Benchmark but no TestResult (should fail)
	xmlContent := `<?xml version="1.0"?>
<Benchmark xmlns="http://checklists.nist.gov/xccdf/1.2">
</Benchmark>`

	test.MockGlobal(t, &check.ParseOSRelease, func(osReleasePath string) (*check.OSRelease, error) {
		return &check.OSRelease{
			ID:           "rhel",
			VersionID:    "9.0",
			Version:      "9.0 (Plow)",
			MajorVersion: 9,
		}, nil
	})

	test.MockGlobal(t, &check.ReadFile, func(filename string) ([]byte, error) {
		if filename == "results.xml" {
			return []byte(xmlContent), nil
		}
		return os.ReadFile(filename)
	})

	test.MockGlobal(t, &check.Exec, func(name string, arg ...string) ([]byte, []byte, int, error) {
		cmd := joinArgs(name, arg...)
		if strings.HasPrefix(cmd, "sudo oscap") {
			return []byte(""), nil, 0, nil
		}
		if strings.HasPrefix(cmd, "sudo chown") {
			return []byte(""), nil, 0, nil
		}
		return nil, nil, 0, nil
	})

	chk, found := check.FindCheckByName("oscap")
	require.True(t, found, "OpenSCAP Check not found")
	config := buildConfig(&blueprint.Customizations{
		OpenSCAP: &blueprint.OpenSCAPCustomization{
			ProfileID:  "xccdf_org.ssgproject.content_profile_ospp",
			DataStream: "/usr/share/xml/scap/ssg/content/ssg-rhel9-ds.xml",
		},
	})

	err := chk.Func(chk.Meta, config)
	require.Error(t, err)
	assert.True(t, check.IsFail(err))
	assert.Contains(t, err.Error(), "expected exactly one test result")
}

func TestOpenSCAPCheckFailMultipleTestResults(t *testing.T) {
	test.MockGlobal(t, &check.Exists, func(name string) bool {
		return name == "results.xml"
	})

	// Mock XML file content with multiple TestResult elements (should fail)
	xmlContent := `<?xml version="1.0"?>
<Benchmark xmlns="http://checklists.nist.gov/xccdf/1.2">
	<TestResult>
		<score>85.0</score>
		<rule-result severity="high" idref="rule1">
			<result>pass</result>
		</rule-result>
	</TestResult>
	<TestResult>
		<score>90.0</score>
		<rule-result severity="high" idref="rule2">
			<result>pass</result>
		</rule-result>
	</TestResult>
</Benchmark>`

	test.MockGlobal(t, &check.ParseOSRelease, func(osReleasePath string) (*check.OSRelease, error) {
		return &check.OSRelease{
			ID:           "rhel",
			VersionID:    "9.0",
			Version:      "9.0 (Plow)",
			MajorVersion: 9,
		}, nil
	})

	test.MockGlobal(t, &check.ReadFile, func(filename string) ([]byte, error) {
		if filename == "results.xml" {
			return []byte(xmlContent), nil
		}
		return os.ReadFile(filename)
	})

	test.MockGlobal(t, &check.Exec, func(name string, arg ...string) ([]byte, []byte, int, error) {
		cmd := joinArgs(name, arg...)
		if strings.HasPrefix(cmd, "sudo oscap") {
			return []byte(""), nil, 0, nil
		}
		if strings.HasPrefix(cmd, "sudo chown") {
			return []byte(""), nil, 0, nil
		}
		return nil, nil, 0, nil
	})

	chk, found := check.FindCheckByName("oscap")
	require.True(t, found, "OpenSCAP Check not found")
	config := buildConfig(&blueprint.Customizations{
		OpenSCAP: &blueprint.OpenSCAPCustomization{
			ProfileID:  "xccdf_org.ssgproject.content_profile_ospp",
			DataStream: "/usr/share/xml/scap/ssg/content/ssg-rhel9-ds.xml",
		},
	})

	err := chk.Func(chk.Meta, config)
	require.Error(t, err)
	assert.True(t, check.IsFail(err))
	assert.Contains(t, err.Error(), "expected exactly one test result")
}
