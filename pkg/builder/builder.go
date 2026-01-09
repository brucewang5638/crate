package builder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"crate/pkg/config"
)

// Build 执行指定目标的构建流程
func Build(cfg *config.Config, target string, force bool) error {
	targets, err := cfg.FindComponents(target)
	if err != nil {
		return err
	}

	fmt.Printf("🎯 目标 '%s' 包含 %d 个构建块，开始构建...\n", target, len(targets))

	for _, comp := range targets {
		if err := buildOne(cfg, comp, force); err != nil {
			return fmt.Errorf("构建构建块 %s 失败: %w", comp.Name, err)
		}
	}

	return nil
}

// buildOne 处理单个构建块的完整生命周期
func buildOne(cfg *config.Config, comp config.Component, force bool) error {
	fmt.Printf("\n🔧 [构建中] %s (v%s-%s)...\n", comp.Name, comp.Version, comp.Edition)

	cacheDir := expandPath(cfg.CacheDir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("无法创建缓存目录: %w", err)
	}

	// 0.1 基础验证 (SourceDir 必须存在)
	if comp.SourceDir != "" {
		if _, err := os.Stat(comp.SourceDir); os.IsNotExist(err) {
			return fmt.Errorf("❌ 源码目录不存在: %s", comp.SourceDir)
		}
		// 0.2 额外文件验证
		for _, file := range comp.ValidateExists {
			checkPath := filepath.Join(comp.SourceDir, file)
			if _, err := os.Stat(checkPath); os.IsNotExist(err) {
				return fmt.Errorf("❌ 缺少必要文件: %s", checkPath)
			}
		}
	}

	// 0.3 执行预构建脚本 (Pre-Build)
	if comp.PreBuild != "" {
		fmt.Printf("   ⚡ 执行预构建脚本: %s\n", comp.PreBuild)
		cmd := exec.Command("sh", "-c", comp.PreBuild)
		cmd.Dir = comp.SourceDir // 在源码目录中执行
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Println(string(out))
			return fmt.Errorf("预构建脚本失败: %w", err)
		}
	}

	// 0.4 检查缓存
	// 匹配模式: name-version-edition*.rpm
	// 这样可以区分 edition (例如 core vs pro)
	cachePattern := fmt.Sprintf("*%s-%s-%s*.rpm", comp.Name, comp.Version, comp.Edition)

	if !force {
		matches, _ := filepath.Glob(filepath.Join(cacheDir, cachePattern))
		if len(matches) > 0 {
			fmt.Printf("   📦 命中缓存: 发现 %d 个 RPM (%s)\n", len(matches), comp.Name)
			// 复制到 RPMS 目录
			rpmDir := filepath.Join(expandPath(cfg.BuildRoot), "RPMS")
			os.MkdirAll(rpmDir, 0755)
			for _, m := range matches {
				dest := filepath.Join(rpmDir, filepath.Base(m))
				copyFile(m, dest)
			}
			return nil
		}
	} else {
		fmt.Printf("   💪 强制模式: 忽略现有缓存，重新构建...\n")
	}

	// 1. 准备环境目录
	buildRoot := expandPath(cfg.BuildRoot)
	rpmDirs := []string{"BUILD", "BUILDROOT", "RPMS", "SOURCES", "SPECS", "SRPMS"}
	for _, d := range rpmDirs {
		if err := os.MkdirAll(filepath.Join(buildRoot, d), 0755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", d, err)
		}
	}

	// 2. 准备源码包 (Create Tarball)
	if comp.SourceDir != "" {
		tarName := fmt.Sprintf("%s.tar.gz", comp.Name)
		tarPath := filepath.Join(buildRoot, "SOURCES", tarName)

		fmt.Printf("   📦 打包源码: %s -> %s\n", comp.SourceDir, tarName)
		// assuming createTarball handles errors correctly
		if err := createTarball(tarPath, comp.SourceDir, comp.Includes, comp.Excludes); err != nil {
			return fmt.Errorf("源码打包失败: %w", err)
		}
	}

	// 3. 准备 SPEC 文件
	specSrc := comp.Spec
	if specSrc == "" {
		return fmt.Errorf("构建块 %s 未定义 spec 文件", comp.Name)
	}
	specDest := filepath.Join(buildRoot, "SPECS", filepath.Base(specSrc))
	if err := copyFile(specSrc, specDest); err != nil {
		return fmt.Errorf("复制 spec 文件失败: %w", err)
	}

	// 4. 执行 rpmbuild
	fmt.Printf("   🔨 执行 rpmbuild...\n")

	args := []string{
		"-ba", specDest,
		"--define", fmt.Sprintf("_topdir %s", buildRoot),
		"--define", fmt.Sprintf("version %s", comp.Version),
		"--define", fmt.Sprintf("dist .%s", cfg.Dist),
		"--define", fmt.Sprintf("comp_name %s", comp.Name),
		"--define", fmt.Sprintf("edition %s", comp.Edition), // 使用 comp.Edition
		"--target", cfg.Arch,
	}

	cmd := exec.Command("rpmbuild", args...)

	// 将输出重定向到标准输出，以便用户看到此时此刻的进度 (避免"卡死"假象)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rpmbuild 执行失败: %w", err)
	}

	// 5. 更新缓存
	// 查找在 RPMS 目录下生成的新 RPM
	builtRPMs, _ := filepath.Glob(filepath.Join(buildRoot, "RPMS", cfg.Arch, cachePattern))
	// 同时也检查 noarch 目录
	builtNoarch, _ := filepath.Glob(filepath.Join(buildRoot, "RPMS", "noarch", cachePattern))
	builtRPMs = append(builtRPMs, builtNoarch...)

	if len(builtRPMs) == 0 {
		return fmt.Errorf("构建看起来成功了，但未找到符合模式 %s 的 RPM 产物", cachePattern)
	}

	for _, rpm := range builtRPMs {
		copyFile(rpm, filepath.Join(cacheDir, filepath.Base(rpm)))
	}
	fmt.Printf("   ✅ 构建成功，已缓存 %d 个 RPM\n", len(builtRPMs))

	return nil
}

func createTarball(tarPath, srcDir string, includes, excludes []string) error {
	// 使用 'tar' 命令进行打包

	var args []string

	if len(includes) > 0 {
		// 模式 A: 指定了 Includes (显式包含)
		// 行为: 进入 srcDir，仅打包 includes 列出的文件/目录
		// 结果: tar 包根目录下直接就是 include1, include2...
		// 在此模式下，Excludes 相对于 srcDir，直接使用即可
		for _, pattern := range excludes {
			args = append(args, fmt.Sprintf("--exclude=%s", pattern))
		}

		args = append(args, "-czf", tarPath, "-C", srcDir)
		args = append(args, includes...)
		fmt.Printf("      (包含: %v, 排除: %v)\n", includes, excludes)
	} else {
		// 模式 B: 默认行为 (打包整个目录)
		// 行为: 进入 srcDir 的父目录，打包 srcDir 本身
		// 结果: tar 包根目录下有一个顶层目录 (srcDir 的 basename)
		parent := filepath.Dir(srcDir)
		base := filepath.Base(srcDir)

		// 在此模式下，tar 包内的路径都以 base/ 开头
		// 因此，用户提供的 exclude 模式如果是路径相关的 (含 /)，需要加上 base/ 前缀
		// 如果只是简单的文件名匹配 (如 *.log)，则不需要加 (tar 默认匹配 basename)
		var adaptedExcludes []string
		for _, pattern := range excludes {
			// 策略:
			// 1. 如果包含路径分隔符 (如 "logs/"), 说明用户意图是匹配特定路径 -> 补全前缀 "base/logs/"
			// 2. 如果不包含 (如 "*.log"), 说明是通配 -> 保持原样 (tar 会递归匹配所有同名文件)
			// 注意: 这里简单判断 "/"，在 Linux 下有效
			if strings.Contains(pattern, "/") {
				// 使用 Join 拼接，保证路径格式正确
				adapted := filepath.Join(base, pattern)
				adaptedExcludes = append(adaptedExcludes, adapted)
				args = append(args, fmt.Sprintf("--exclude=%s", adapted))
			} else {
				adaptedExcludes = append(adaptedExcludes, pattern)
				args = append(args, fmt.Sprintf("--exclude=%s", pattern))
			}
		}

		args = append(args, "-czf", tarPath, "-C", parent, base)
		if len(adaptedExcludes) > 0 {
			fmt.Printf("      (包含: [全部], 排除: %v)\n", adaptedExcludes)
		}
	}

	cmd := exec.Command("tar", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar 命令失败: %s (%w)", string(out), err)
	}
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
	expanded := path
	if strings.HasPrefix(path, "~/") {
		dirname, _ := os.UserHomeDir()
		expanded = filepath.Join(dirname, path[2:])
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return expanded // fallback
	}
	return abs
}
