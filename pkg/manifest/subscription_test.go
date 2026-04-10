package manifest

import (
	"fmt"
	"testing"

	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/stretchr/testify/assert"
)

const (
	// env file path where the activation key is written, used in multiple places
	subkeyFilepath = "/etc/osbuild-subscription-register.env"
)

func TestSubscriptionService(t *testing.T) {
	type testCase struct {
		subOpts          subscription.ImageOptions
		srvcOpts         *subscriptionServiceOptions
		expectedStage    *osbuild.Stage
		expectedDirs     []*fsnode.Directory
		expectedFiles    []*fsnode.File
		expectedServices []string
	}

	// values that are always set for the stage
	stageType := "org.osbuild.systemd.unit.create"
	serviceFilename := "osbuild-subscription-register.service"
	unitType := osbuild.SystemUnitType
	serviceDescription := "First-boot service for registering with Red Hat subscription manager and/or insights"
	serviceWants := []string{"network-online.target"}
	serviceAfter := []string{"selinux-autorelabel.service", "network-online.target"}
	serviceWantedBy := []string{"default.target"}

	testCases := map[string]testCase{
		"simple": {
			subOpts: subscription.ImageOptions{
				Organization:  "theorg",
				ActivationKey: "thekey",
				ServerUrl:     "theserverurl",
				BaseUrl:       "thebaseurl",
				Insights:      false,
				Rhc:           false,
			},
			srvcOpts: nil,
			expectedStage: &osbuild.Stage{
				Type: stageType,
				Options: &osbuild.SystemdUnitCreateStageOptions{
					Filename: serviceFilename,
					UnitType: unitType,
					UnitPath: osbuild.UsrUnitPath,
					Config: osbuild.SystemdUnit{
						Unit: &osbuild.UnitSection{
							Description: serviceDescription,
							ConditionPathExists: []string{
								subkeyFilepath,
							},
							Wants: serviceWants,
							After: serviceAfter,
						},
						Service: &osbuild.ServiceSection{
							Type: osbuild.OneshotServiceType,
							ExecStart: []string{
								"/usr/sbin/subscription-manager config --server.hostname 'theserverurl'",
								`/usr/sbin/subscription-manager register --org="${ORG_ID}" --activationkey="${ACTIVATION_KEY}" --baseurl 'thebaseurl'`,
								"/usr/bin/rm '" + subkeyFilepath + "'",
							},
							EnvironmentFile: []string{
								subkeyFilepath,
							},
						},
						Install: &osbuild.InstallSection{
							WantedBy: serviceWantedBy,
						},
					},
				},
			},
			expectedFiles:    []*fsnode.File{mkKeyfile("theorg", "thekey")},
			expectedDirs:     make([]*fsnode.Directory, 0),
			expectedServices: []string{serviceFilename},
		},
		"with-insights": {
			subOpts: subscription.ImageOptions{
				Organization:  "theorg-wi",
				ActivationKey: "thekey-wi",
				ServerUrl:     "theserverurl-wi",
				BaseUrl:       "thebaseurl-wi",
				Insights:      true,
				Rhc:           false,
			},
			srvcOpts: nil,
			expectedStage: &osbuild.Stage{
				Type: stageType,
				Options: &osbuild.SystemdUnitCreateStageOptions{
					Filename: serviceFilename,
					UnitType: unitType,
					UnitPath: osbuild.UsrUnitPath,
					Config: osbuild.SystemdUnit{
						Unit: &osbuild.UnitSection{
							Description: serviceDescription,
							ConditionPathExists: []string{
								subkeyFilepath,
							},
							Wants: serviceWants,
							After: serviceAfter,
						},
						Service: &osbuild.ServiceSection{
							Type: osbuild.OneshotServiceType,
							ExecStart: []string{
								"/usr/sbin/subscription-manager config --server.hostname 'theserverurl-wi'",
								`/usr/sbin/subscription-manager register --org="${ORG_ID}" --activationkey="${ACTIVATION_KEY}" --baseurl 'thebaseurl-wi'`,
								"/usr/bin/insights-client --register", // added when insights is enabled
								"/usr/bin/rm '" + subkeyFilepath + "'",
							},
							EnvironmentFile: []string{
								subkeyFilepath,
							},
						},
						Install: &osbuild.InstallSection{
							WantedBy: serviceWantedBy,
						},
					},
				},
			},
			expectedFiles:    []*fsnode.File{mkKeyfile("theorg-wi", "thekey-wi")},
			expectedDirs:     make([]*fsnode.Directory, 0),
			expectedServices: []string{serviceFilename},
		},
		"with-insights-template": {
			subOpts: subscription.ImageOptions{
				Organization:  "theorg-wi",
				ActivationKey: "thekey-wi",
				ServerUrl:     "theserverurl-wi",
				BaseUrl:       "thebaseurl-wi",
				Insights:      true,
				Rhc:           false,
				TemplateUUID:  "template-uuid",
				Proxy:         "https://proxy-url",
				PatchURL:      "https://cert.console.redhat.com/api/patch/v3/",
			},
			srvcOpts: nil,
			expectedStage: &osbuild.Stage{
				Type: stageType,
				Options: &osbuild.SystemdUnitCreateStageOptions{
					Filename: serviceFilename,
					UnitType: unitType,
					UnitPath: osbuild.UsrUnitPath,
					Config: osbuild.SystemdUnit{
						Unit: &osbuild.UnitSection{
							Description: serviceDescription,
							ConditionPathExists: []string{
								subkeyFilepath,
							},
							Wants: serviceWants,
							After: serviceAfter,
						},
						Service: &osbuild.ServiceSection{
							Type: osbuild.OneshotServiceType,
							ExecStart: []string{
								"/usr/sbin/subscription-manager config --server.hostname 'theserverurl-wi'",
								"/usr/sbin/subscription-manager config --server.proxy_scheme 'https'",
								"/usr/sbin/subscription-manager config --server.proxy_hostname 'proxy-url'",
								`/usr/sbin/subscription-manager register --org="${ORG_ID}" --activationkey="${ACTIVATION_KEY}" --baseurl 'thebaseurl-wi'`,
								"/usr/bin/insights-client --register", // added when insights is enabled
								"curl -v --retry 5 --cert /etc/pki/consumer/cert.pem --key /etc/pki/consumer/key.pem -X PATCH 'https://cert.console.redhat.com/api/patch/v3/templates/template-uuid/subscribed-systems' --proxy 'https://proxy-url'",
								"/usr/sbin/subscription-manager refresh",
								"/usr/bin/rm '" + subkeyFilepath + "'",
							},
							EnvironmentFile: []string{
								subkeyFilepath,
							},
						},
						Install: &osbuild.InstallSection{
							WantedBy: serviceWantedBy,
						},
					},
				},
			},
			expectedFiles:    []*fsnode.File{mkKeyfile("theorg-wi", "thekey-wi")},
			expectedDirs:     make([]*fsnode.Directory, 0),
			expectedServices: []string{serviceFilename},
		},
		"with-rhc": {
			subOpts: subscription.ImageOptions{
				Organization:  "theorg-wr",
				ActivationKey: "thekey-wr",
				ServerUrl:     "theserverurl-wr",
				BaseUrl:       "thebaseurl-wr",
				Insights:      false,
				Rhc:           true,
			},
			srvcOpts: &subscriptionServiceOptions{
				PermissiveRHC: true,
			},
			expectedStage: &osbuild.Stage{
				Type: stageType,
				Options: &osbuild.SystemdUnitCreateStageOptions{
					Filename: serviceFilename,
					UnitType: unitType,
					UnitPath: osbuild.UsrUnitPath,
					Config: osbuild.SystemdUnit{
						Unit: &osbuild.UnitSection{
							Description: serviceDescription,
							ConditionPathExists: []string{
								subkeyFilepath,
							},
							Wants: serviceWants,
							After: serviceAfter,
						},
						Service: &osbuild.ServiceSection{
							Type: osbuild.OneshotServiceType,
							ExecStart: []string{
								"/usr/sbin/subscription-manager config --server.hostname 'theserverurl-wr'",
								`/usr/bin/rhc connect --organization="${ORG_ID}" --activation-key="${ACTIVATION_KEY}"`,
								"/usr/sbin/semanage permissive --add rhcd_t", // added when rhc is enabled
								"/usr/bin/rm '" + subkeyFilepath + "'",
							},
							EnvironmentFile: []string{
								subkeyFilepath,
							},
						},
						Install: &osbuild.InstallSection{
							WantedBy: serviceWantedBy,
						},
					},
				},
			},
			expectedFiles:    []*fsnode.File{mkKeyfile("theorg-wr", "thekey-wr")},
			expectedDirs:     make([]*fsnode.Directory, 0),
			expectedServices: []string{serviceFilename},
		},
		"with-insights-rhc": { // rhc also covers insights, so this test case has the same general result as above
			subOpts: subscription.ImageOptions{
				Organization:  "theorg-wir",
				ActivationKey: "thekey-wir",
				ServerUrl:     "theserverurl-wir",
				BaseUrl:       "thebaseurl-wir",
				Insights:      true,
				Rhc:           true,
			},
			srvcOpts: &subscriptionServiceOptions{
				PermissiveRHC: true,
			},
			expectedStage: &osbuild.Stage{
				Type: stageType,
				Options: &osbuild.SystemdUnitCreateStageOptions{
					Filename: serviceFilename,
					UnitType: unitType,
					UnitPath: osbuild.UsrUnitPath,
					Config: osbuild.SystemdUnit{
						Unit: &osbuild.UnitSection{
							Description: serviceDescription,
							ConditionPathExists: []string{
								subkeyFilepath,
							},
							Wants: serviceWants,
							After: serviceAfter,
						},
						Service: &osbuild.ServiceSection{
							Type: osbuild.OneshotServiceType,
							ExecStart: []string{
								"/usr/sbin/subscription-manager config --server.hostname 'theserverurl-wir'",
								`/usr/bin/rhc connect --organization="${ORG_ID}" --activation-key="${ACTIVATION_KEY}"`,
								"/usr/sbin/semanage permissive --add rhcd_t", // added when rhc is enabled
								"/usr/bin/rm '" + subkeyFilepath + "'",
							},
							EnvironmentFile: []string{
								subkeyFilepath,
							},
						},
						Install: &osbuild.InstallSection{
							WantedBy: serviceWantedBy,
						},
					},
				},
			},
			expectedFiles:    []*fsnode.File{mkKeyfile("theorg-wir", "thekey-wir")},
			expectedDirs:     make([]*fsnode.Directory, 0),
			expectedServices: []string{serviceFilename},
		},
		"with-rhc-no-permissive": {
			subOpts: subscription.ImageOptions{
				Organization:  "theorg-wr",
				ActivationKey: "thekey-wr",
				ServerUrl:     "theserverurl-wr",
				BaseUrl:       "thebaseurl-wr",
				Insights:      false,
				Rhc:           true,
			},
			srvcOpts: &subscriptionServiceOptions{
				PermissiveRHC: false,
			},
			expectedStage: &osbuild.Stage{
				Type: stageType,
				Options: &osbuild.SystemdUnitCreateStageOptions{
					Filename: serviceFilename,
					UnitType: unitType,
					UnitPath: osbuild.UsrUnitPath,
					Config: osbuild.SystemdUnit{
						Unit: &osbuild.UnitSection{
							Description: serviceDescription,
							ConditionPathExists: []string{
								subkeyFilepath,
							},
							Wants: serviceWants,
							After: serviceAfter,
						},
						Service: &osbuild.ServiceSection{
							Type: osbuild.OneshotServiceType,
							ExecStart: []string{
								"/usr/sbin/subscription-manager config --server.hostname 'theserverurl-wr'",
								"/usr/bin/rhc connect --organization=\"${ORG_ID}\" --activation-key=\"${ACTIVATION_KEY}\"",
								"/usr/bin/rm '" + subkeyFilepath + "'",
							},
							EnvironmentFile: []string{
								subkeyFilepath,
							},
						},
						Install: &osbuild.InstallSection{
							WantedBy: serviceWantedBy,
						},
					},
				},
			},
			expectedFiles:    []*fsnode.File{mkKeyfile("theorg-wr", "thekey-wr")},
			expectedDirs:     make([]*fsnode.Directory, 0),
			expectedServices: []string{serviceFilename},
		},
		"with-insights-rhc-template-name": {
			subOpts: subscription.ImageOptions{
				Organization:  "theorg-wir",
				ActivationKey: "thekey-wir",
				ServerUrl:     "theserverurl-wir",
				BaseUrl:       "thebaseurl-wir",
				Insights:      true,
				Rhc:           true,
				TemplateName:  "template name",
			},
			srvcOpts: &subscriptionServiceOptions{
				PermissiveRHC: true,
			},
			expectedStage: &osbuild.Stage{
				Type: stageType,
				Options: &osbuild.SystemdUnitCreateStageOptions{
					Filename: serviceFilename,
					UnitType: unitType,
					UnitPath: osbuild.UsrUnitPath,
					Config: osbuild.SystemdUnit{
						Unit: &osbuild.UnitSection{
							Description: serviceDescription,
							ConditionPathExists: []string{
								subkeyFilepath,
							},
							Wants: serviceWants,
							After: serviceAfter,
						},
						Service: &osbuild.ServiceSection{
							Type: osbuild.OneshotServiceType,
							ExecStart: []string{
								"/usr/sbin/subscription-manager config --server.hostname 'theserverurl-wir'",
								`/usr/bin/rhc connect --organization="${ORG_ID}" --activation-key="${ACTIVATION_KEY}" --content-template='template name'`,
								"/usr/sbin/semanage permissive --add rhcd_t", // added when rhc is enabled
								"/usr/bin/rm '" + subkeyFilepath + "'",
							},
							EnvironmentFile: []string{
								subkeyFilepath,
							},
						},
						Install: &osbuild.InstallSection{
							WantedBy: serviceWantedBy,
						},
					},
				},
			},
			expectedFiles:    []*fsnode.File{mkKeyfile("theorg-wir", "thekey-wir")},
			expectedDirs:     make([]*fsnode.Directory, 0),
			expectedServices: []string{serviceFilename},
		},
		"with-insights-rhc-template-uuid": {
			subOpts: subscription.ImageOptions{
				Organization:  "theorg-wir",
				ActivationKey: "thekey-wir",
				ServerUrl:     "theserverurl-wir",
				BaseUrl:       "thebaseurl-wir",
				Insights:      true,
				Rhc:           true,
				TemplateName:  "template-name",
				TemplateUUID:  "template-uuid",
				Proxy:         "proxy-url:8080",
				PatchURL:      "https://cert.console.redhat.com/api/patch/v3/",
			},
			srvcOpts: &subscriptionServiceOptions{
				PermissiveRHC: true,
			},
			expectedStage: &osbuild.Stage{
				Type: stageType,
				Options: &osbuild.SystemdUnitCreateStageOptions{
					Filename: serviceFilename,
					UnitType: unitType,
					UnitPath: osbuild.UsrUnitPath,
					Config: osbuild.SystemdUnit{
						Unit: &osbuild.UnitSection{
							Description: serviceDescription,
							ConditionPathExists: []string{
								subkeyFilepath,
							},
							Wants: serviceWants,
							After: serviceAfter,
						},
						Service: &osbuild.ServiceSection{
							Type: osbuild.OneshotServiceType,
							ExecStart: []string{
								"/usr/sbin/subscription-manager config --server.hostname 'theserverurl-wir'",
								"/usr/sbin/subscription-manager config --server.proxy_scheme 'http'",
								"/usr/sbin/subscription-manager config --server.proxy_hostname 'proxy-url'",
								"/usr/sbin/subscription-manager config --server.proxy_port '8080'",
								`/usr/bin/rhc connect --organization="${ORG_ID}" --activation-key="${ACTIVATION_KEY}"`,
								"/usr/sbin/semanage permissive --add rhcd_t", // added when rhc is enabled
								"curl -v --retry 5 --cert /etc/pki/consumer/cert.pem --key /etc/pki/consumer/key.pem -X PATCH 'https://cert.console.redhat.com/api/patch/v3/templates/template-uuid/subscribed-systems' --proxy 'proxy-url:8080'",
								"/usr/sbin/subscription-manager refresh",
								"/usr/bin/rm '" + subkeyFilepath + "'",
							},
							EnvironmentFile: []string{
								subkeyFilepath,
							},
						},
						Install: &osbuild.InstallSection{
							WantedBy: serviceWantedBy,
						},
					},
				},
			},
			expectedFiles:    []*fsnode.File{mkKeyfile("theorg-wir", "thekey-wir")},
			expectedDirs:     make([]*fsnode.Directory, 0),
			expectedServices: []string{serviceFilename},
		},
		"insights-on-boot": {
			subOpts: subscription.ImageOptions{
				Organization:  "theorg-iob",
				ActivationKey: "thekey-iob",
				ServerUrl:     "theserverurl-iob",
				BaseUrl:       "thebaseurl-iob",
				Insights:      true,
				Rhc:           false,
			},
			srvcOpts: &subscriptionServiceOptions{
				InsightsOnBoot: true,
			},
			expectedStage: &osbuild.Stage{
				Type: stageType,
				Options: &osbuild.SystemdUnitCreateStageOptions{
					Filename: serviceFilename,
					UnitType: unitType,
					UnitPath: osbuild.UsrUnitPath,
					Config: osbuild.SystemdUnit{
						Unit: &osbuild.UnitSection{
							Description: serviceDescription,
							ConditionPathExists: []string{
								subkeyFilepath,
							},
							Wants: serviceWants,
							After: serviceAfter,
						},
						Service: &osbuild.ServiceSection{
							Type: osbuild.OneshotServiceType,
							ExecStart: []string{
								"/usr/sbin/subscription-manager config --server.hostname 'theserverurl-iob'",
								`/usr/sbin/subscription-manager register --org="${ORG_ID}" --activationkey="${ACTIVATION_KEY}" --baseurl 'thebaseurl-iob'`,
								"/usr/bin/insights-client --register", // added when insights is enabled
								"/usr/bin/rm '" + subkeyFilepath + "'",
							},
							EnvironmentFile: []string{
								subkeyFilepath,
							},
						},
						Install: &osbuild.InstallSection{
							WantedBy: serviceWantedBy,
						},
					},
				},
			},
			expectedFiles: []*fsnode.File{
				mkKeyfile("theorg-iob", "thekey-iob"),
				mkInsightsDropinFile(),
			},
			expectedDirs:     []*fsnode.Directory{mkInsightsDropinDir()},
			expectedServices: []string{serviceFilename},
		},
		"etc-unit-path": {
			subOpts: subscription.ImageOptions{
				Organization:  "theorg-etc",
				ActivationKey: "thekey-etc",
				ServerUrl:     "theserverurl-etc",
				BaseUrl:       "thebaseurl-etc",
				Insights:      false,
				Rhc:           false,
			},
			srvcOpts: &subscriptionServiceOptions{
				InsightsOnBoot: true,
				UnitPath:       osbuild.EtcUnitPath,
			},
			expectedStage: &osbuild.Stage{
				Type: stageType,
				Options: &osbuild.SystemdUnitCreateStageOptions{
					Filename: serviceFilename,
					UnitType: unitType,
					UnitPath: osbuild.EtcUnitPath,
					Config: osbuild.SystemdUnit{
						Unit: &osbuild.UnitSection{
							Description: serviceDescription,
							ConditionPathExists: []string{
								subkeyFilepath,
							},
							Wants: serviceWants,
							After: serviceAfter,
						},
						Service: &osbuild.ServiceSection{
							Type: osbuild.OneshotServiceType,
							ExecStart: []string{
								"/usr/sbin/subscription-manager config --server.hostname 'theserverurl-etc'",
								`/usr/sbin/subscription-manager register --org="${ORG_ID}" --activationkey="${ACTIVATION_KEY}" --baseurl 'thebaseurl-etc'`,
								"/usr/bin/rm '" + subkeyFilepath + "'",
							},
							EnvironmentFile: []string{
								subkeyFilepath,
							},
						},
						Install: &osbuild.InstallSection{
							WantedBy: serviceWantedBy,
						},
					},
				},
			},
			expectedFiles: []*fsnode.File{
				mkKeyfile("theorg-etc", "thekey-etc"),
			},
			expectedDirs:     make([]*fsnode.Directory, 0),
			expectedServices: []string{serviceFilename},
		},
		"insights-on-boot+etc-unit-path": {
			subOpts: subscription.ImageOptions{
				Organization:  "theorg-iob-etc",
				ActivationKey: "thekey-iob-etc",
				ServerUrl:     "theserverurl-iob-etc",
				BaseUrl:       "thebaseurl-iob-etc",
				Insights:      true,
				Rhc:           false,
			},
			srvcOpts: &subscriptionServiceOptions{
				InsightsOnBoot: true,
				UnitPath:       osbuild.EtcUnitPath,
			},
			expectedStage: &osbuild.Stage{
				Type: stageType,
				Options: &osbuild.SystemdUnitCreateStageOptions{
					Filename: serviceFilename,
					UnitType: unitType,
					UnitPath: osbuild.EtcUnitPath,
					Config: osbuild.SystemdUnit{
						Unit: &osbuild.UnitSection{
							Description: serviceDescription,
							ConditionPathExists: []string{
								subkeyFilepath,
							},
							Wants: serviceWants,
							After: serviceAfter,
						},
						Service: &osbuild.ServiceSection{
							Type: osbuild.OneshotServiceType,
							ExecStart: []string{
								"/usr/sbin/subscription-manager config --server.hostname 'theserverurl-iob-etc'",
								`/usr/sbin/subscription-manager register --org="${ORG_ID}" --activationkey="${ACTIVATION_KEY}" --baseurl 'thebaseurl-iob-etc'`,
								"/usr/bin/insights-client --register", // added when insights is enabled
								"/usr/bin/rm '" + subkeyFilepath + "'",
							},
							EnvironmentFile: []string{
								subkeyFilepath,
							},
						},
						Install: &osbuild.InstallSection{
							WantedBy: serviceWantedBy,
						},
					},
				},
			},
			expectedFiles: []*fsnode.File{
				mkKeyfile("theorg-iob-etc", "thekey-iob-etc"),
				mkInsightsDropinFile(),
			},
			expectedDirs:     []*fsnode.Directory{mkInsightsDropinDir()},
			expectedServices: []string{serviceFilename},
		},
		"with-proxy-port": {
			subOpts: subscription.ImageOptions{
				Organization:  "theorg-pp",
				ActivationKey: "thekey-pp",
				ServerUrl:     "theserverurl-pp",
				BaseUrl:       "thebaseurl-pp",
				Insights:      false,
				Rhc:           false,
				Proxy:         "http://proxy-url:8080",
			},
			srvcOpts: nil,
			expectedStage: &osbuild.Stage{
				Type: stageType,
				Options: &osbuild.SystemdUnitCreateStageOptions{
					Filename: serviceFilename,
					UnitType: unitType,
					UnitPath: osbuild.UsrUnitPath,
					Config: osbuild.SystemdUnit{
						Unit: &osbuild.UnitSection{
							Description: serviceDescription,
							ConditionPathExists: []string{
								subkeyFilepath,
							},
							Wants: serviceWants,
							After: serviceAfter,
						},
						Service: &osbuild.ServiceSection{
							Type: osbuild.OneshotServiceType,
							ExecStart: []string{
								"/usr/sbin/subscription-manager config --server.hostname 'theserverurl-pp'",
								"/usr/sbin/subscription-manager config --server.proxy_scheme 'http'",
								"/usr/sbin/subscription-manager config --server.proxy_hostname 'proxy-url'",
								"/usr/sbin/subscription-manager config --server.proxy_port '8080'",
								"/usr/sbin/subscription-manager register --org=\"${ORG_ID}\" --activationkey=\"${ACTIVATION_KEY}\" --baseurl 'thebaseurl-pp'",
								"/usr/bin/rm '" + subkeyFilepath + "'",
							},
							EnvironmentFile: []string{
								subkeyFilepath,
							},
						},
						Install: &osbuild.InstallSection{
							WantedBy: serviceWantedBy,
						},
					},
				},
			},
			expectedFiles: []*fsnode.File{
				mkKeyfile("theorg-pp", "thekey-pp"),
			},
			expectedDirs:     make([]*fsnode.Directory, 0),
			expectedServices: []string{serviceFilename},
		},
		"with-content-sets": {
			subOpts: subscription.ImageOptions{
				Organization:  "theorg-iob-etc",
				ActivationKey: "thekey-iob-etc",
				ServerUrl:     "theserverurl-iob-etc",
				BaseUrl:       "thebaseurl-iob-etc",
				Rhc:           false,
				Insights:      false,
				ContentSets:   []string{"content-label-1", "content-label-2"},
			},
			srvcOpts: &subscriptionServiceOptions{
				InsightsOnBoot: true,
				UnitPath:       osbuild.EtcUnitPath,
			},
			expectedStage: &osbuild.Stage{
				Type: stageType,
				Options: &osbuild.SystemdUnitCreateStageOptions{
					Filename: serviceFilename,
					UnitType: unitType,
					UnitPath: osbuild.EtcUnitPath,
					Config: osbuild.SystemdUnit{
						Unit: &osbuild.UnitSection{
							Description: serviceDescription,
							ConditionPathExists: []string{
								subkeyFilepath,
							},
							Wants: serviceWants,
							After: serviceAfter,
						},
						Service: &osbuild.ServiceSection{
							Type: osbuild.OneshotServiceType,
							ExecStart: []string{
								"/usr/sbin/subscription-manager config --server.hostname 'theserverurl-iob-etc'",
								`/usr/sbin/subscription-manager register --org="${ORG_ID}" --activationkey="${ACTIVATION_KEY}" --baseurl 'thebaseurl-iob-etc'`,
								"/usr/sbin/subscription-manager repos --enable='content-label-1' --enable='content-label-2'",
								"/usr/bin/rm '" + subkeyFilepath + "'",
							},
							EnvironmentFile: []string{
								subkeyFilepath,
							},
						},
						Install: &osbuild.InstallSection{
							WantedBy: serviceWantedBy,
						},
					},
				},
			},
			expectedFiles: []*fsnode.File{
				mkKeyfile("theorg-iob-etc", "thekey-iob-etc"),
			},
			expectedDirs:     make([]*fsnode.Directory, 0),
			expectedServices: []string{serviceFilename},
		},
	}

	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			stage, dirs, files, services, err := subscriptionService(tc.subOpts, tc.srvcOpts)
			assert.NoError(err)
			assert.Equal(stage, tc.expectedStage)
			assert.Equal(dirs, tc.expectedDirs)
			assert.Equal(files, tc.expectedFiles)
			assert.Equal(services, tc.expectedServices)

			// ensure no directories or files have non-nil ownership
			for _, file := range files {
				assert.Nil(file.User())
				assert.Nil(file.Group())
			}

			for _, dir := range dirs {
				assert.Nil(dir.User())
				assert.Nil(dir.Group())
			}
		})
	}
}

func mkKeyfile(org, key string) *fsnode.File {
	file, err := fsnode.NewFile(subkeyFilepath, nil, nil, nil, []byte(fmt.Sprintf("ORG_ID=%s\nACTIVATION_KEY=%s", org, key)))
	if err != nil {
		panic(err)
	}

	return file
}

func mkInsightsDropinFile() *fsnode.File {
	dropinContents := `[Unit]
Requisite=greenboot-healthcheck.service
After=network-online.target greenboot-healthcheck.service osbuild-first-boot.service osbuild-subscription-register.service
ConditionPathExists=
[Service]
ExecStartPre=
[Install]
WantedBy=multi-user.target
`
	icDropinFile, err := fsnode.NewFile("/etc/systemd/system/insights-client-boot.service.d/override.conf", nil, nil, nil, []byte(dropinContents))
	if err != nil {
		panic(err)
	}
	return icDropinFile
}

func mkInsightsDropinDir() *fsnode.Directory {
	icDropinDirectory, err := fsnode.NewDirectory("/etc/systemd/system/insights-client-boot.service.d", nil, nil, nil, true)
	if err != nil {
		panic(err)
	}
	return icDropinDirectory
}
