package main

import (
	"fmt"
	"os"
	"reflect"

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

		// If no value is defined in the env try the default value
		confV, ok := os.LookupEnv(key)
		if !ok {
			confV, ok = fieldT.Tag.Lookup("default")
		}

		kind := fieldV.Kind()
		if ok {
			switch kind {
			case reflect.Ptr:
				fieldV.Set(reflect.ValueOf(&confV))
			case reflect.String:
				fieldV.SetString(confV)
			default:
				return fmt.Errorf("Unsupported type")
			}
		} else if kind == reflect.String {
			return fmt.Errorf("Undefined non-pointer field without default value %v", fieldT)
		}
	}
	return nil
}

func main() {
	var config ImageBuilderConfig
	err := LoadConfigFromEnv(&config)
	if err != nil {
		panic(err)
	}

	log, err := logger.NewLogger(config.LogLevel, config.AccessKeyID, config.SecretAccessKey, config.Region, config.LogGroup)
	if err != nil {
		panic(err)
	}

	s := server.NewServer(log)
	s.Run(config.ListenAddress)
}
