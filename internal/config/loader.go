package config

import (
	"fmt"
	"os"

	"github.com/ersinkoc/tentaserve/internal/config/yaml"
)

// Load reads a YAML configuration file and returns a parsed Config.
// It performs environment variable interpolation and validation.
func Load(path string) (*Config, error) {
	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Perform environment variable interpolation
	content := yaml.InterpolateEnv(string(data))

	// Parse YAML
	raw, err := yaml.ParseString(content)
	if err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Create config with defaults
	cfg := DefaultConfig()

	// Unmarshal into struct
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Apply post-unmarshal fixes for duration and memory fields
	if err := fixConfigFields(cfg, raw); err != nil {
		return nil, fmt.Errorf("fixing config fields: %w", err)
	}

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

// LoadOrDefault loads the configuration file if it exists, otherwise returns defaults.
func LoadOrDefault(path string) (*Config, error) {
	if _, err := os.Stat(path); err != nil {
		// File doesn't exist, return defaults
		cfg := DefaultConfig()
		return cfg, nil
	}
	return Load(path)
}

// fixConfigFields applies post-unmarshal fixes for fields that need special handling.
// This includes parsing memory size strings.
func fixConfigFields(cfg *Config, raw map[string]any) error {
	// Fix cache size fields
	if gateway, ok := raw["gateway"].(map[string]any); ok {
		if cache, ok := gateway["cache"].(map[string]any); ok {
			if v, ok := cache["max_size"].(string); ok && v != "" {
				size, err := yaml.ParseMemorySize(v)
				if err != nil {
					return fmt.Errorf("parsing cache.max_size: %w", err)
				}
				cfg.Gateway.Cache.MaxSize = size
			}
			if v, ok := cache["max_entry_size"].(string); ok && v != "" {
				size, err := yaml.ParseMemorySize(v)
				if err != nil {
					return fmt.Errorf("parsing cache.max_entry_size: %w", err)
				}
				cfg.Gateway.Cache.MaxEntrySize = size
			}
		}
	}

	return nil
}
