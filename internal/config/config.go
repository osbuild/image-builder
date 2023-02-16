package config

import "strings"

// Do not write this config to logs or stdout, it contains secrets!
type ImageBuilderConfig struct {
	ListenAddress        string `env:"LISTEN_ADDRESS"`
	LogLevel             string `env:"LOG_LEVEL"`
	LogGroup             string `env:"CW_LOG_GROUP"`
	CwRegion             string `env:"CW_AWS_REGION"`
	CwAccessKeyID        string `env:"CW_AWS_ACCESS_KEY_ID"`
	CwSecretAccessKey    string `env:"CW_AWS_SECRET_ACCESS_KEY"`
	ComposerURL          string `env:"COMPOSER_URL"`
	ComposerTokenURL     string `env:"COMPOSER_TOKEN_URL"`
	ComposerClientId     string `env:"COMPOSER_CLIENT_ID"`
	ComposerOfflineToken string `env:"COMPOSER_OFFLINE_TOKEN"`
	ComposerClientSecret string `env:"COMPOSER_CLIENT_SECRET"`
	ComposerCA           string `env:"COMPOSER_CA_PATH"`
	OsbuildRegion        string `env:"OSBUILD_AWS_REGION"`
	OsbuildGCPRegion     string `env:"OSBUILD_GCP_REGION"`
	OsbuildGCPBucket     string `env:"OSBUILD_GCP_BUCKET"`
	DistributionsDir     string `env:"DISTRIBUTIONS_DIR"`
	MigrationsDir        string `env:"MIGRATIONS_DIR"`
	TernExecutable       string `env:"TERN_EXECUTABLE"`
	TernMigrationsDir    string `env:"TERN_MIGRATIONS_DIR"`
	PGHost               string `env:"PGHOST"`
	PGPort               string `env:"PGPORT"`
	PGDatabase           string `env:"PGDATABASE"`
	PGUser               string `env:"PGUSER"`
	PGPassword           string `env:"PGPASSWORD"`
	PGSSLMode            string `env:"PGSSLMODE"`
	QuotaFile            string `env:"QUOTA_FILE"`
	AllowFile            string `env:"ALLOW_FILE"`
	SplunkHost           string `env:"SPLUNK_HEC_HOST"`
	SplunkPort           string `env:"SPLUNK_HEC_PORT"`
	SplunkToken          string `env:"SPLUNK_HEC_TOKEN"`
	ProvisioningURL      string `env:"PROVISIONING_URL"`
}

func (ibc *ImageBuilderConfig) IsDebug() bool {
	level := strings.ToUpper(ibc.LogLevel)
	return level == "TRACE" || level == "DEBUG"
}
