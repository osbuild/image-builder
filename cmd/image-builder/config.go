package main

// Do not write this config to logs or stdout, it contains secrets!
type ImageBuilderConfig struct {
	ListenAddress          string  `env:"LISTEN_ADDRESS"`
	LogLevel               string  `env:"LOG_LEVEL"`
	LogGroup               *string `env:"CW_LOG_GROUP"`
	CwRegion               *string `env:"CW_AWS_REGION"`
	CwAccessKeyID          *string `env:"CW_AWS_ACCESS_KEY_ID"`
	CwSecretAccessKey      *string `env:"CW_AWS_SECRET_ACCESS_KEY"`
	OsbuildRegion          string  `env:"OSBUILD_AWS_REGION"`
	OsbuildAccessKeyID     string  `env:"OSBUILD_AWS_ACCESS_KEY_ID"`
	OsbuildSecretAccessKey string  `env:"OSBUILD_AWS_SECRET_ACCESS_KEY"`
	OsbuildS3Bucket        string  `env:"OSBUILD_AWS_S3_BUCKET"`
	OsbuildURL             string  `env:"OSBUILD_URL"`
	OsbuildCert            *string `env:"OSBUILD_CERT_PATH"`
	OsbuildKey             *string `env:"OSBUILD_KEY_PATH"`
	OsbuildCA              *string `env:"OSBUILD_CA_PATH"`
	OrgIds                 string  `env:"ALLOWED_ORG_IDS"`
	DistributionsDir       string  `env:"DISTRIBUTIONS_DIR"`
}
