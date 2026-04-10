package check

import (
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/gocomply/scap/pkg/scap/models/cdf"
	"github.com/osbuild/images/internal/buildconfig"
)

// ignoredSeverityRules is a list of rule IDs that are ignored by the OpenSCAP check.
var ignoredSeverityRules = []string{
	"xccdf_org.ssgproject.content_rule_ensure_redhat_gpgkey_installed", // requires rhsm subscription
}

func init() {
	RegisterCheck(Metadata{
		Name:                   "oscap",
		RequiresBlueprint:      true,
		RequiresCustomizations: true,
	}, openSCAPCheck)
}

// GetDatastreamFilename returns the full OpenSCAP datastream path based on OSRelease.
// Returns the full path (e.g., "/usr/share/xml/scap/ssg/content/ssg-rhel9-ds.xml") or an error if the OS/version combination is not supported.
func GetDatastreamFilename(release *OSRelease) (string, error) {
	// Map of OS ID and version to datastream filenames
	datastreamMap := map[string]string{
		"rhel:8":    "ssg-rhel8-ds.xml",
		"rhel:9":    "ssg-rhel9-ds.xml",
		"rhel:10":   "ssg-rhel10-ds.xml",
		"centos:8":  "ssg-centos8-ds.xml",
		"centos:9":  "ssg-cs9-ds.xml",
		"centos:10": "ssg-cs10-ds.xml",
		"fedora":    "ssg-fedora-ds.xml",
	}

	// Build lookup key
	var key string
	switch release.ID {
	case "rhel", "centos":
		if release.MajorVersion == 0 {
			return "", fmt.Errorf("unsupported OS version: %s %s", release.ID, release.VersionID)
		}
		key = fmt.Sprintf("%s:%d", release.ID, release.MajorVersion)
	case "fedora":
		key = "fedora"
	default:
		return "", fmt.Errorf("unsupported OS ID: %s", release.ID)
	}

	filename, ok := datastreamMap[key]
	if !ok {
		return "", fmt.Errorf("no datastream found for %s version %d", release.ID, release.MajorVersion)
	}

	return "/usr/share/xml/scap/ssg/content/" + filename, nil
}

// parseOSCAPResults parses the OpenSCAP results.xml file and extracts the score and rule results
func parseOSCAPResults(filename string) (score string, failedHighSeverityRules []string, err error) {
	data, err := ReadFile(filename)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read results.xml: %w", err)
	}
	log.Printf("Read file: %s (%d bytes)\n", filename, len(data))

	var benchmark cdf.Benchmark
	if err := xml.Unmarshal(data, &benchmark); err != nil {
		return "", nil, fmt.Errorf("failed to parse XML (only XCCDF 1.2 is supported): %w", err)
	}

	if len(benchmark.TestResult) != 1 {
		return "", nil, fmt.Errorf("expected exactly one test result, found %d", len(benchmark.TestResult))
	}

	testResult := benchmark.TestResult[0]

	if len(testResult.Score) == 0 {
		return "", nil, fmt.Errorf("score not found in results.xml")
	}
	score = strings.TrimSpace(testResult.Score[0].Text)
	if score == "" {
		return "", nil, fmt.Errorf("score is empty in results.xml")
	}

	for _, rule := range testResult.RuleResult {
		severityStr := string(rule.Severity)
		resultStr := string(rule.Result)

		if severityStr == "high" && strings.ToLower(resultStr) == "fail" {
			if !slices.Contains(ignoredSeverityRules, rule.Idref) {
				ruleStr := fmt.Sprintf("rule-result idref=\"%s\" severity=\"%s\" result=\"%s\"", rule.Idref, severityStr, resultStr)
				failedHighSeverityRules = append(failedHighSeverityRules, ruleStr)
			}
		}
	}

	return score, failedHighSeverityRules, nil
}

func openSCAPCheck(meta *Metadata, config *buildconfig.BuildConfig) error {
	oscap := config.Blueprint.Customizations.OpenSCAP
	if oscap == nil {
		return Skip("no OpenSCAP customization")
	}

	osRelease, err := ParseOSRelease("/etc/os-release")
	if err != nil {
		return Fail("failed to read OS ID from /etc/os-release:", err)
	}

	if osRelease.ID == "rhel" && osRelease.MajorVersion < 8 {
		return Skip("only XCCDF 1.2 is supported, which requires RHEL 8.0+")
	}

	baselineScore := 0.8
	profile := oscap.ProfileID
	datastream := oscap.DataStream

	if profile == "" {
		return Skip("incomplete OpenSCAP configuration")
	}

	// Handle null/empty datastream by finding default datastream
	// See pkg/customizations/oscap/oscap.go:datastream fallbacks
	if datastream == "" || datastream == "null" {
		datastream, err = GetDatastreamFilename(osRelease)
		if err != nil {
			return Fail("failed to determine datastream filename:", err)
		}

		log.Printf("Using default datastream: %s\n", datastream)
	}

	profileName := profile + "_osbuild_tailoring"

	// Run oscap evaluation
	// NOTE: sudo works here without password because we test this only on ami
	// initialised with cloud-init, which sets sudo NOPASSWD for the user
	// NOTE: oscap returns exit code 2 for any failed rules, so we ignore the error
	stdout, _, _, err := Exec("sudo", "oscap", "xccdf", "eval",
		"--results", "results.xml",
		"--profile", profileName,
		"--tailoring-file", "/oscap_data/tailoring.xml",
		datastream)

	// oscap may return non-zero exit code even on success (exit code 2 for failed rules)
	// so we check if results.xml was created instead
	if !Exists("results.xml") {
		return Fail("oscap evaluation failed:", string(stdout), "error:", err)
	}

	_, _, _, err = Exec("sudo", "chown", fmt.Sprintf("%d", os.Getuid()), "results.xml")
	if err != nil {
		log.Printf("Warning: failed to chown results.xml: %v\n", err)
	}

	scoreStr, failedRules, err := parseOSCAPResults("results.xml")
	if err != nil {
		return Fail("failed to parse results.xml:", err)
	}

	hardenedScore, err := strconv.ParseFloat(scoreStr, 64)
	if err != nil {
		return Fail("failed to parse oscap score:", scoreStr, "error:", err)
	}
	// XCCDF scores are already normalized between 0 and 1
	// If score is > 1, it's a percentage that needs conversion
	if hardenedScore > 1.0 {
		hardenedScore = hardenedScore / 100.0
	}

	log.Printf("Hardened score: %.2f%%\n", hardenedScore*100)

	severityCount := len(failedRules)
	log.Printf("Severity count: %d\n", severityCount)

	log.Printf("Baseline score: %.2f%%\n", baselineScore*100)
	log.Printf("Hardened score: %.2f%%\n", hardenedScore*100)

	if hardenedScore < baselineScore {
		return Fail("hardened image score (", hardenedScore*100,
			"%) did not improve baseline score (", baselineScore*100, "%)")
	}

	if severityCount > 0 {
		log.Println("Failed high severity rules:")
		for _, rule := range failedRules {
			log.Printf("  %s\n", rule)
		}
		return Fail("one or more oscap rules with high severity failed")
	}

	return Pass()
}
