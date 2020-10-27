package main

import (
	"fmt"
	"os"
	"reflect"

	"github.com/osbuild/image-builder/internal/cloudapi"
	"github.com/osbuild/image-builder/internal/logger"
	"github.com/osbuild/image-builder/internal/server"
)

// *string means the value is not required
// string means the value is required and should have a default value
func LoadConfigFromEnv(intf interface{}) error {
	t := reflect.TypeOf(intf).Elem()
	v := reflect.ValueOf(intf).Elem()

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
			case reflect.Ptr:
				if fieldT.Type.Elem().Kind() != reflect.String {
					return fmt.Errorf("Unsupported type")
				}
				fieldV.Set(reflect.ValueOf(&confV))
			case reflect.String:
				fieldV.SetString(confV)
			default:
				return fmt.Errorf("Unsupported type")
			}
		}
	}
	return nil
}

func main() {
	config := ImageBuilderConfig{
		ListenAddress: "localhost:8086",
		LogLevel:      "INFO",
	}

	err := LoadConfigFromEnv(&config)
	if err != nil {
		panic(err)
	}

	log, err := logger.NewLogger(config.LogLevel, config.AccessKeyID, config.SecretAccessKey, config.Region, config.LogGroup)
	if err != nil {
		panic(err)
	}

	client := cloudapi.NewOsbuildClient(config.OsbuildURL, config.OsbuildCert, config.OsbuildKey, config.OsbuildCA)

	s := server.NewServer(log, client)
	s.Run(config.ListenAddress)
}
