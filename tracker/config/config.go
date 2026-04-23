package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Embedder  EmbedderConfig  `yaml:"embedder"`
	Generator GeneratorConfig `yaml:"generator"`
	Search    SearchConfig    `yaml:"search"`
}

type EmbedderConfig struct {
	OllamaURL string `yaml:"ollama_url"`
	Model     string `yaml:"model"`
}

type GeneratorConfig struct {
	OllamaURL string `yaml:"ollama_url"`
	Model     string `yaml:"model"`
}

type SearchConfig struct {
	CandidateK          int     `yaml:"candidate_k"`
	FinalN              int     `yaml:"final_n"`
	RerankEnabled       bool    `yaml:"rerank_enabled"`
	RerankAlpha         float32 `yaml:"rerank_alpha"`
	QueryRewriteEnabled bool    `yaml:"query_rewrite_enabled"`
	ExplainEnabled      bool    `yaml:"explain_enabled"`
}

func Defaults() Config {
	return Config{
		Embedder: EmbedderConfig{
			OllamaURL: "http://localhost:11434",
			Model:     "bge-m3",
		},
		Generator: GeneratorConfig{
			OllamaURL: "http://localhost:11434",
			Model:     "qwen2.5:3b",
		},
		Search: SearchConfig{
			CandidateK:          20,
			FinalN:              5,
			RerankEnabled:       true,
			RerankAlpha:         0.3,
			QueryRewriteEnabled: true,
			ExplainEnabled:      true,
		},
	}
}

func (c *Config) SetConfigs() {
	d := Defaults()

	if c.Embedder.OllamaURL == "" {
		c.Embedder.OllamaURL = d.Embedder.OllamaURL
	}
	if c.Embedder.Model == "" {
		c.Embedder.Model = d.Embedder.Model
	}

	if c.Generator.OllamaURL == "" {
		c.Generator.OllamaURL = d.Generator.OllamaURL
	}

	if c.Search.CandidateK <= 0 {
		c.Search.CandidateK = d.Search.CandidateK
	}
	if c.Search.FinalN <= 0 {
		c.Search.FinalN = d.Search.FinalN
	}
	if c.Search.FinalN > c.Search.CandidateK {
		c.Search.FinalN = c.Search.CandidateK
	}
	if c.Search.RerankAlpha < 0 || c.Search.RerankAlpha > 1 {
		c.Search.RerankAlpha = d.Search.RerankAlpha
	}

	if c.Generator.Model == "" {
		c.Search.RerankEnabled = false
		c.Search.QueryRewriteEnabled = false
		c.Search.ExplainEnabled = false
	}
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

	cfg.SetConfigs()
	return &cfg, nil
}
