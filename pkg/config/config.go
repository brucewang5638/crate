package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

type Component struct {
	Name           string   `yaml:"name"`
	Version        string   `yaml:"version"`
	Type           string   `yaml:"type"`            // 类型: infra, service, app
	Enabled        *bool    `yaml:"enabled"`         // 是否启用构建 (默认: true)
	Edition        string   `yaml:"edition"`         // [新增] 版本标识 (Release/Variant)，如 "1", "core", "pro"
	Spec           string   `yaml:"spec"`            // SPEC 文件路径
	SourceDir      string   `yaml:"source_dir"`      // 源码目录绝对路径
	Includes       []string `yaml:"includes"`        // 仅打包指定的子文件/目录 (相对 SourceDir)
	PreBuild       string   `yaml:"pre_build"`       // 预构建脚本 (Shell 命令)
	ValidateExists []string `yaml:"validate_exists"` // 必须存在的文件列表 (相对 SourceDir)
	Tags           []string `yaml:"tags"`
}

type Config struct {
	ProjectName string              `yaml:"project_name"`
	DistDefault string              `yaml:"dist_default"`
	CacheDir    string              `yaml:"cache_dir"`
	BuildRoot   string              `yaml:"build_root"`
	Components  []Component         `yaml:"components"`
	Groups      map[string][]string `yaml:"groups"`
	SystemDeps  []string            `yaml:"system_deps"`

	// 运行时字段
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

	// 设置默认值
	if cfg.Arch == "" {
		// 自动检测架构
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

	// 4. 处理相对路径
	// 如果配置文件路径不是空的，计算它的基础目录
	if absPath, err := filepath.Abs(path); err == nil {
		baseDir := filepath.Dir(absPath)
		for i := range cfg.Components {
			// 默认 Enabled 为 true
			if cfg.Components[i].Enabled == nil {
				defaultEnabled := true
				cfg.Components[i].Enabled = &defaultEnabled
			}

			// 如果 Spec 是相对路径，将其拼接为基于 config 的绝对路径
			if cfg.Components[i].Spec != "" && !filepath.IsAbs(cfg.Components[i].Spec) {
				cfg.Components[i].Spec = filepath.Join(baseDir, cfg.Components[i].Spec)
			}
			// 如果 SourceDir 是相对路径 (虽然通常推荐绝对路径)，也进行转换
			if cfg.Components[i].SourceDir != "" && !filepath.IsAbs(cfg.Components[i].SourceDir) {
				cfg.Components[i].SourceDir = filepath.Join(baseDir, cfg.Components[i].SourceDir)
			}
		}
	}

	return &cfg, nil
}

// FindComponents 将目标字符串（组名或构建块名）展开为构建块列表
func (c *Config) FindComponents(target string) ([]Component, error) {
	// 0. 特殊关键字: all
	if target == "all" {
		var active []Component
		for _, comp := range c.Components {
			if comp.Enabled != nil && *comp.Enabled {
				active = append(active, comp)
			}
		}
		return active, nil
	}

	// 1. 检查是否为组名
	if members, ok := c.Groups[target]; ok {
		var result []Component
		for _, m := range members {
			// 递归逻辑（如果组包含组）目前暂未实现
			// 假设组内只包含标签 (Tags) 或构建块名 (Name)

			// 尝试按名称或标签查找构建块
			found := c.findByTagOrName(m)
			result = append(result, found...)
		}
		return unique(result), nil
	}

	// 2. 检查是否为直接的构建块名
	found := c.findByTagOrName(target)
	if len(found) > 0 {
		return found, nil
	}

	return nil, fmt.Errorf("未找到目标 '%s'", target)
}

func (c *Config) findByTagOrName(key string) []Component {
	var result []Component
	for _, comp := range c.Components {
		// 1. 如果是直接通过 Name 精确匹配，即使 Disabled 也允许构建 (显式调用)
		if comp.Name == key {
			result = append(result, comp)
			continue
		}

		// 2. 如果是通过 Tags 查找，则必须检查 Enabled 状态
		// 跳过未启用的构建块
		if comp.Enabled != nil && !*comp.Enabled {
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
