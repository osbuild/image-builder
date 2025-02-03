package main

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
)

// Do not write this config to logs or stdout, it contains secrets!
type Config struct {
	DryRun                  bool   `env:"DRY_RUN"`
	EnableDBMaintenance     bool   `env:"ENABLE_DB_MAINTENANCE"`
	ComposesRetentionMonths int    `env:"DB_COMPOSES_RETENTION_MONTHS"`
	PGHost                  string `env:"PGHOST"`
	PGPort                  string `env:"PGPORT"`
	PGDatabase              string `env:"PGDATABASE"`
	PGUser                  string `env:"PGUSER"`
	PGPassword              string `env:"PGPASSWORD"`
	PGSSLMode               string `env:"PGSSLMODE"`
}

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
			return fmt.Errorf("no env tag in config field")
		}

		confV, ok := os.LookupEnv(key)
		kind := fieldV.Kind()
		if ok {
			switch kind {
			case reflect.String:
				fieldV.SetString(confV)
			case reflect.Int:
				value, err := strconv.ParseInt(confV, 10, 64)
				if err != nil {
					return err
				}
				fieldV.SetInt(value)
			case reflect.Bool:
				value, err := strconv.ParseBool(confV)
				if err != nil {
					return err
				}
				fieldV.SetBool(value)
			default:
				return fmt.Errorf("unsupported type")
			}
		}
	}
	return nil
}
