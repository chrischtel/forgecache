package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// ForgeConfig represents the structure of a .forgefile
type ForgeConfig struct {
	Toolchain ToolchainConfig `toml:"toolchain"`
	Build     BuildConfig     `toml:"build"`
	Cache     CacheConfig     `toml:"cache"`
}

// ToolchainConfig defines language and tool versions
type ToolchainConfig struct {
	Go   string `toml:"go,omitempty"`
	Zig  string `toml:"zig,omitempty"`
	Rust string `toml:"rust,omitempty"`
}

// BuildConfig defines how to build the project
type BuildConfig struct {
	Cmd string `toml:"cmd"`
}

// CacheConfig defines what to cache
type CacheConfig struct {
	Inputs  []string `toml:"inputs"`
	Outputs []string `toml:"outputs"`
}

// LoadConfig loads the .forgefile from the current directory
func LoadConfig() (*ForgeConfig, error) {
	configPath := filepath.Join(".", ".forgefile")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, err
	}

	var config ForgeConfig
	if _, err := toml.DecodeFile(configPath, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// SaveConfig saves the config to a .forgefile
func SaveConfig(config *ForgeConfig) error {
	configPath := filepath.Join(".", ".forgefile")

	file, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	return encoder.Encode(config)
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *ForgeConfig {
	return &ForgeConfig{
		Toolchain: ToolchainConfig{
			Go: "1.22",
		},
		Build: BuildConfig{
			Cmd: "go build",
		},
		Cache: CacheConfig{
			Inputs:  []string{"go.mod", "go.sum", "**/*.go"},
			Outputs: []string{"./"},
		},
	}
}
