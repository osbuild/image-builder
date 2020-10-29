package main

// Do not write this config to logs or stdout, it contains secrets!
type ImageBuilderConfig struct {
	ListenAddress   string  `env:"LISTEN_ADDRESS" default:"localhost:8086"`
	LogLevel        string  `env:"LOG_LEVEL" default:"INFO"`
	LogGroup        *string `env:"CW_LOG_GROUP"`
	Region          *string `env:"CW_AWS_REGION"`
	AccessKeyID     *string `env:"CW_AWS_ACCESS_KEY_ID"`
	SecretAccessKey *string `env:"CW_AWS_SECRET_ACCESS_KEY"`
}
