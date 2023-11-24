package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/joho/godotenv"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
)

var ErrMissingEnvTag = errors.New("missing 'env' tag in config field")
var ErrUnsupportedFieldType = errors.New("unsupported config field type")

func LoadConfigFromEnv(conf *ImageBuilderConfig) error {
	err := godotenv.Load("local.env")
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to load local.env file: %w", err)
	}

	t := reflect.TypeOf(conf).Elem()
	v := reflect.ValueOf(conf).Elem()

	for i := 0; i < v.NumField(); i++ {
		fieldT := t.Field(i)
		fieldV := v.Field(i)
		key, ok := fieldT.Tag.Lookup("env")
		if !ok {
			return ErrMissingEnvTag
		}

		confV, ok := os.LookupEnv(key)
		kind := fieldV.Kind()
		if ok {
			switch kind {
			case reflect.String:
				fieldV.SetString(confV)
			default:
				return ErrUnsupportedFieldType
			}
		}
	}

	// Load variables if running as a ClowdApp
	if clowder.IsClowderEnabled() {
		conf.PGHost = clowder.LoadedConfig.Database.Hostname
		conf.PGDatabase = clowder.LoadedConfig.Database.Name
		conf.PGUser = clowder.LoadedConfig.Database.Username
		conf.PGPassword = clowder.LoadedConfig.Database.Password

		if clowder.LoadedConfig.Logging.Cloudwatch != nil {
			conf.CwRegion = clowder.LoadedConfig.Logging.Cloudwatch.Region
			conf.CwAccessKeyID = clowder.LoadedConfig.Logging.Cloudwatch.AccessKeyId
			conf.CwSecretAccessKey = clowder.LoadedConfig.Logging.Cloudwatch.SecretAccessKey
			conf.LogGroup = clowder.LoadedConfig.Logging.Cloudwatch.LogGroup
		}

		if endpoint, ok := clowder.DependencyEndpoints["provisioning-backend"]["api"]; ok {
			conf.ProvisioningURL = fmt.Sprintf("http://%s:%d/api/provisioning/v1", endpoint.Hostname, endpoint.Port)
		}
	}

	return nil
}
