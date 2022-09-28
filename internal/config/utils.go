package config

import (
	"fmt"
	"os"
	"reflect"

	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
)

func LoadConfigFromEnv(conf *ImageBuilderConfig) error {
	t := reflect.TypeOf(conf).Elem()
	v := reflect.ValueOf(conf).Elem()

	for i := 0; i < v.NumField(); i++ {
		fieldT := t.Field(i)
		fieldV := v.Field(i)
		key, ok := fieldT.Tag.Lookup("env")
		if !ok {
			return fmt.Errorf("No env tag in config field")
		}

		confV, ok := os.LookupEnv(key)
		kind := fieldV.Kind()
		if ok {
			switch kind {
			case reflect.String:
				fieldV.SetString(confV)
			default:
				return fmt.Errorf("Unsupported type")
			}
		}
	}

	// Load database variables if running in ephemeral environment
	if clowder.IsClowderEnabled() {
		conf.PGHost = clowder.LoadedConfig.Database.Hostname
		conf.PGDatabase = clowder.LoadedConfig.Database.Name
		conf.PGUser = clowder.LoadedConfig.Database.Username
		conf.PGPassword = clowder.LoadedConfig.Database.Password
	}

	return nil
}
