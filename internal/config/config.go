package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Pool    PoolConfig    `yaml:"pool"`
	VM      VMConfig      `yaml:"vm"`
	Network NetworkConfig `yaml:"network"`
	API     APIConfig     `yaml:"api"`
}

type PoolConfig struct {
	WarmSize int `yaml:"warmSize"`
	MaxVMs   int `yaml:"maxVMs"`
}

type VMConfig struct {
	CPUs      int    `yaml:"cpus"`
	MemoryGiB int    `yaml:"memoryGiB"`
	DiskGiB   int    `yaml:"diskGiB"`
	Master    string `yaml:"master"`
}

type NetworkConfig struct {
	Domain       string `yaml:"domain"`
	RegistryPort int    `yaml:"registryPort"`
	TraefikHTTP  int    `yaml:"traefikHTTP"`
	TraefikHTTPS int    `yaml:"traefikHTTPS"`
}

type APIConfig struct {
	Port int `yaml:"port"`
}

func Default() Config {
	return Config{
		Pool: PoolConfig{
			WarmSize: 3,
			MaxVMs:   15,
		},
		VM: VMConfig{
			CPUs:      2,
			MemoryGiB: 3,
			DiskGiB:   30,
			Master:    "agent-master",
		},
		Network: NetworkConfig{
			Domain:       "agents.test",
			RegistryPort: 8090,
			TraefikHTTP:  80,
			TraefikHTTPS: 443,
		},
		API: APIConfig{
			Port: 8091,
		},
	}
}

func BaseDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".agentvm")
}

func ConfigPath() string {
	return filepath.Join(BaseDir(), "config.yaml")
}

func Load() (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}

func Save(cfg Config) error {
	if err := os.MkdirAll(BaseDir(), 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(), data, 0644)
}

func EnsureDirs() error {
	dirs := []string{
		BaseDir(),
		filepath.Join(BaseDir(), "shared", "bin"),
		filepath.Join(BaseDir(), "traefik", "dynamic"),
		filepath.Join(BaseDir(), "certs"),
		filepath.Join(BaseDir(), "logs"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", d, err)
		}
	}
	return nil
}
