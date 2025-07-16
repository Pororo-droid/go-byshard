package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"
)

var (
	configInstance *Config
	configOnce     sync.Once
)

type Config struct {
	Bootstrap BootstrapConfig `json:"bootstrap"`
}

type BootstrapConfig struct {
	IP   string `json:"IP"`
	Port int    `json:"Port"`
}

func GetConfig() *Config {
	configOnce.Do(func() {
		var err error
		configInstance, err = LoadConfig()
		if err != nil {
			log.Fatalf("Failed loading config: %v", err)
		}
	})
	return configInstance
}

func GetBootstrapConfig() *BootstrapConfig {
	configOnce.Do(func() {
		var err error
		configInstance, err = LoadConfig()
		if err != nil {
			log.Fatalf("Failed loading config: %v", err)
		}
	})
	return &configInstance.Bootstrap
}

func LoadConfig() (*Config, error) {
	file, err := os.Open("../config/config.json")
	if err != nil {
		return nil, fmt.Errorf("error while opening json file: %v", err)
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error while reading json file: %v", err)
	}

	var config Config
	if err := json.Unmarshal(bytes, &config); err != nil {
		return nil, fmt.Errorf("error while parsing json file: %v", err)
	}

	return &config, nil
}
