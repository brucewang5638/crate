package config

import (
	"fmt"
	"os"
	"runtime"

	"gopkg.in/yaml.v3"
)

type Component struct {
	Name      string   `yaml:"name"`
	Version   string   `yaml:"version"`
	Type      string   `yaml:"type"`
	Spec      string   `yaml:"spec"`
	SourceDir string   `yaml:"source_dir"`
	Tags      []string `yaml:"tags"`
}

type Config struct {
	ProjectName string              `yaml:"project_name"`
	DistDefault string              `yaml:"dist_default"`
	CacheDir    string              `yaml:"cache_dir"`
	BuildRoot   string              `yaml:"build_root"`
	Components  []Component         `yaml:"components"`
	Groups      map[string][]string `yaml:"groups"`
	SystemDeps  []string            `yaml:"system_deps"`

	// Runtime fields
	Arch string `yaml:"-"`
	Dist string `yaml:"-"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Set defaults
	if cfg.Arch == "" {
		// Detect architecture
		switch runtime.GOARCH {
		case "amd64":
			cfg.Arch = "x86_64"
		case "arm64":
			cfg.Arch = "aarch64"
		default:
			cfg.Arch = runtime.GOARCH
		}
	}
	if cfg.Dist == "" {
		cfg.Dist = cfg.DistDefault
	}

	return &cfg, nil
}

// FindComponents expands a target string (group name or component name) into a list of components
func (c *Config) FindComponents(target string) ([]Component, error) {
	// 1. Check if it's a group
	if members, ok := c.Groups[target]; ok {
		var result []Component
		for _, m := range members {
			// Recursive logic isn't strictly needed if groups only contain tags or names
			// But for now, let's assume groups map to Tags or Names
			// Strategy: 'members' in group can be other groups or tags or names.
			// Simplified: Groups map to TAGS or NAMES.

			// Try to find components by Name or Tag matching 'm'
			found := c.findByTagOrName(m)
			result = append(result, found...)
		}
		return unique(result), nil
	}

	// 2. Check if it's a direct component name
	found := c.findByTagOrName(target)
	if len(found) > 0 {
		return found, nil
	}

	return nil, fmt.Errorf("target '%s' not found", target)
}

func (c *Config) findByTagOrName(key string) []Component {
	var result []Component
	for _, comp := range c.Components {
		if comp.Name == key {
			result = append(result, comp)
			continue
		}
		for _, t := range comp.Tags {
			if t == key {
				result = append(result, comp)
				break
			}
		}
	}
	return result
}

func unique(comps []Component) []Component {
	keys := make(map[string]bool)
	var list []Component
	for _, entry := range comps {
		if _, value := keys[entry.Name]; !value {
			keys[entry.Name] = true
			list = append(list, entry)
		}
	}
	return list
}
