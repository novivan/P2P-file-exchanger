package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Tracker  TrackerConfig  `yaml:"tracker"`
	Server   ServerConfig   `yaml:"server"`
	Download DownloadConfig `yaml:"download"`
}

type TrackerConfig struct {
	URL string `yaml:"url"`
}

type ServerConfig struct {
	APIPort int `yaml:"api_port"`
	P2PPort int `yaml:"p2p_port"`
}

type DownloadConfig struct {
	Workers int `yaml:"workers"`
}

const DefaultDownloadWorkers = 10

func (d *DownloadConfig) WorkersOrDefault() int {
	if d.Workers <= 0 {
		return DefaultDownloadWorkers
	}
	return d.Workers
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}
