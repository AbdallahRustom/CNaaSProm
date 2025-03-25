package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config struct to hold application configuration
type Config struct {
	Server struct {
		Address string `yaml:"address"`
		Port    uint   `yaml:"port"`
	} `yaml:"Server"`

	RemoteStatisticServer struct {
		Address string `yaml:"address"`
		Port    uint   `yaml:"port"`
	} `yaml:"RemoteStatisticServer"`

	RemoteMonitoringServer struct {
		Address string `yaml:"address"`
		Port    uint   `yaml:"port"`
	} `yaml:"RemoteMonitoringServer"`

	MetricsStatisticsCategory []string `yaml:"MetricsStatisticsCategory"`
	MetricsMonitoringCategory []string `yaml:"MetricsMonitoringCategory"`
	QueryParams               string   `yaml:"queryParams"`
}

// LoadConfig loads the YAML configuration file
func LoadConfig(filename string) (*Config, error) {
	config := &Config{}
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %v", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(config); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %v", err)
	}

	return config, nil
}
