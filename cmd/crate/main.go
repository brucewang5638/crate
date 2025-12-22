package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"crate/internal/builder"
	"crate/internal/config"
	"crate/internal/repo"
)

func main() {
	// 定义命令行参数
	configFile := flag.String("config", "config.yaml", "配置文件路径")
	buildTarget := flag.String("build", "", "指定构建的目标组件或组 (例如: 'redis', 'components')")
	release := flag.Bool("release", false, "生成最终发布包 (包含仓库和安装脚本)")
	arch := flag.String("arch", "", "覆盖默认架构 (例如: aarch64)")
	dist := flag.String("dist", "", "覆盖发行版标识 (例如: el7)")

	flag.Parse()

	// 1. 加载配置
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Printf("❌ 加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 命令行参数覆盖配置文件
	if *arch != "" {
		cfg.Arch = *arch
	}
	if *dist != "" {
		cfg.Dist = *dist
	}

	fmt.Printf("🌟 Crate 构建工具 v0.1 | 架构: %s | 发行版: %s\n", cfg.Arch, cfg.Dist)

	// 2. 执行构建任务
	if *buildTarget != "" {
		if err := builder.Build(cfg, *buildTarget); err != nil {
			fmt.Printf("❌ 构建失败: %v\n", err)
			os.Exit(1)
		}
	}

	// 3. 执行发布任务
	if *release {
		if err := runRelease(cfg); err != nil {
			fmt.Printf("❌ 发布失败: %v\n", err)
			os.Exit(1)
		}
	}

	if *buildTarget == "" && !*release {
		// 如果未指定操作，显示帮助信息
		flag.Usage()
	}
}

func runRelease(cfg *config.Config) error {
	releaseName := fmt.Sprintf("%s-%s-%s", cfg.ProjectName, cfg.Dist, time.Now().Format("20060102"))
	releaseDir := filepath.Join(os.Getenv("HOME"), releaseName)

	fmt.Printf("\n🚀 开始生成发布包: %s\n", releaseName)

	// 1. 准备目录
	if err := os.MkdirAll(releaseDir, 0755); err != nil {
		return fmt.Errorf("无法创建发布目录: %w", err)
	}

	// 2. 汇聚 RPM (从缓存或构建目录)
	// 这里我们假设所有需要的 RPM 已经在缓存中 (前提是用户先跑了 --build all 或者之前的构建)
	// 或者，我们应该从 RPMS 目录收集?
	// 简单策略: 从缓存目录收集所有符合当前架构的 RPM 到 releaseDir/repo
	// 但 repo.Generate 期望的是一个目录。
	// 让我们把 RPM 复制到 releaseDir/repo
	repoDir := filepath.Join(releaseDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return err
	}

	cacheDir := expandPath(cfg.CacheDir)
	// 复制所有缓存的 RPM (粗粒度，实际可能需要筛选)
	rpms, _ := filepath.Glob(filepath.Join(cacheDir, "*.rpm"))
	fmt.Printf("   📦 正在汇聚 %d 个 RPM 到仓库...\n", len(rpms))
	for _, rpm := range rpms {
		dst := filepath.Join(repoDir, filepath.Base(rpm))
		if err := copyFile(rpm, dst); err != nil {
			return err
		}
	}

	// 3. 生成仓库和脚本
	if err := repo.Generate(cfg, releaseDir); err != nil {
		return err
	}

	// 4. 生成版本清单 (Manifest)
	manifestPath := filepath.Join(releaseDir, "VERSION_MANIFEST.txt")
	manifestContent := fmt.Sprintf("发布版本: %s\n构建时间: %s\n架构: %s\n\n包含组件:\n",
		releaseName, time.Now().Format(time.RFC3339), cfg.Arch)
	for _, c := range cfg.Components {
		manifestContent += fmt.Sprintf("- %s: %s\n", c.Name, c.Version)
	}
	os.WriteFile(manifestPath, []byte(manifestContent), 0644)

	// 5. 打包最终 tar.gz
	tarball := releaseDir + ".tar.gz"
	fmt.Printf("   🗜️  正在生成最终压缩包: %s ...\n", filepath.Base(tarball))

	// tar -C home -czf releaseName.tar.gz releaseName
	cmd := exec.Command("tar", "-czf", tarball, "-C", filepath.Dir(releaseDir), filepath.Base(releaseDir))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tar 失败: %s", string(out))
	}

	fmt.Printf("\n🎉 发布完成! 文件位于: %s\n", tarball)
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
