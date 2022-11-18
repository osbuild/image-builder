package v1

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type ConfigOption func(*AppConfig)

var LoadedConfig *AppConfig
var KafkaTopics map[string]TopicConfig
var DependencyEndpoints map[string]map[string]DependencyEndpoint
var PrivateDependencyEndpoints map[string]map[string]PrivateDependencyEndpoint
var ObjectBuckets map[string]ObjectStoreBucket
var KafkaServers []string

func loadConfig(filename string) (*AppConfig, error) {
	var appConfig AppConfig
	jsonFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()
	data, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &appConfig)
	if err != nil {
		return nil, err
	}
	return &appConfig, nil
}

func IsClowderEnabled() bool {
	_, ok := os.LookupEnv("ACG_CONFIG")
	return ok
}

func init() {
	if !IsClowderEnabled() {
		return
	}
	loadedConfig, err := loadConfig(os.Getenv("ACG_CONFIG"))
	if err != nil {
		fmt.Println(err)
		return
	}
	LoadedConfig = loadedConfig
	KafkaTopics = make(map[string]TopicConfig)
	if LoadedConfig.Kafka != nil {
		for _, topic := range LoadedConfig.Kafka.Topics {
			KafkaTopics[topic.RequestedName] = topic
		}
	}
	DependencyEndpoints = make(map[string]map[string]DependencyEndpoint)
	if LoadedConfig.Endpoints != nil {
		for _, endpoint := range LoadedConfig.Endpoints {
			if DependencyEndpoints[endpoint.App] == nil {
				DependencyEndpoints[endpoint.App] = make(map[string]DependencyEndpoint)
			}
			DependencyEndpoints[endpoint.App][endpoint.Name] = endpoint
		}
	}

	PrivateDependencyEndpoints = make(map[string]map[string]PrivateDependencyEndpoint)
	if LoadedConfig.PrivateEndpoints != nil {
		for _, endpoint := range LoadedConfig.PrivateEndpoints {
			if PrivateDependencyEndpoints[endpoint.App] == nil {
				PrivateDependencyEndpoints[endpoint.App] = make(map[string]PrivateDependencyEndpoint)
			}
			PrivateDependencyEndpoints[endpoint.App][endpoint.Name] = endpoint
		}
	}

	ObjectBuckets = make(map[string]ObjectStoreBucket)
	if LoadedConfig.ObjectStore != nil {
		for _, bucket := range LoadedConfig.ObjectStore.Buckets {
			ObjectBuckets[bucket.RequestedName] = bucket
		}
	}
	if LoadedConfig.Kafka != nil {
		for _, broker := range LoadedConfig.Kafka.Brokers {
			KafkaServers = append(KafkaServers, fmt.Sprintf("%s:%d", broker.Hostname, *broker.Port))
		}
	}
}

// RdsCa writes the RDS CA from the JSON config to a temporary file and returns
// the path
func (a AppConfig) RdsCa() (string, error) {
	return writeContent("rdsca", "rds", a.Database.RdsCa)
}

// KafkaCa writes the Kafka CA from the JSON config to a temporary file and returns
// the path
func (a AppConfig) KafkaCa(brokers ...BrokerConfig) (string, error) {
	if len(brokers) == 0 {
		if len(LoadedConfig.Kafka.Brokers) == 0 {
			return "", fmt.Errorf("no broker availabl")
		}
		brokers = LoadedConfig.Kafka.Brokers
	}
	return writeContent("kafkaca", "kafka", brokers[0].Cacert)
}

func (a AppConfig) KafkaFirstCa() (string, error) {
	if a.Kafka == nil || len(a.Kafka.Brokers) == 0 || a.Kafka.Brokers[0].Cacert == nil {
		return "", fmt.Errorf("could not find ca for first broker")
	}
	file := a.Kafka.Brokers[0].Cacert
	return writeContent("kafkaca", "kafka", file)
}

func writeContent(dir string, file string, contentString *string) (string, error) {

	dir, err := ioutil.TempDir("", dir)
	if err != nil {
		return "", err
	}

	if contentString == nil {
		return "", fmt.Errorf("no RDS available")
	}

	content := []byte(*contentString)

	fil, err := ioutil.TempFile(dir, file)

	if err != nil {
		return "", err
	}

	if err := ioutil.WriteFile(fil.Name(), content, 0666); err != nil {
		return "", err
	}

	return fil.Name(), nil
}
