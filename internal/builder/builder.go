package builder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"crate/internal/config"
)

// Build 执行指定目标的构建流程
func Build(cfg *config.Config, target string) error {
	targets, err := cfg.FindComponents(target)
	if err != nil {
		return err
	}

	fmt.Printf("🎯 目标 '%s' 包含 %d 个组件，开始构建...\n", target, len(targets))

	for _, comp := range targets {
		if err := buildOne(cfg, comp); err != nil {
			return fmt.Errorf("构建组件 %s 失败: %w", comp.Name, err)
		}
	}

	return nil
}

// buildOne 处理单个组件的完整生命周期
func buildOne(cfg *config.Config, comp config.Component) error {
	fmt.Printf("\n🔧 [构建中] %s (v%s)...\n", comp.Name, comp.Version)

	cacheDir := expandPath(cfg.CacheDir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("无法创建缓存目录: %w", err)
	}

	// 0. 检查缓存 (简单模式匹配)
	// 假设模式: hk-component-name-version*.rpm 或类似
	// For simplicity, we search for *name-version*.rpm
	cachePattern := fmt.Sprintf("*%s-%s*.rpm", comp.Name, comp.Version)
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
		// Assuming createTarball handles errors correctly
		if err := createTarball(tarPath, comp.SourceDir, comp.Name); err != nil {
			return fmt.Errorf("源码打包失败: %w", err)
		}
	}

	// 3. 准备 SPEC 文件
	specSrc := comp.Spec
	if specSrc == "" {
		return fmt.Errorf("组件 %s 未定义 spec 文件", comp.Name)
	}
	specDest := filepath.Join(buildRoot, "SPECS", filepath.Base(specSrc))
	if err := copyFile(specSrc, specDest); err != nil {
		return fmt.Errorf("复制 spec 文件失败: %w", err)
	}

	// 4. 执行 rpmbuild
	fmt.Printf("   🔨 执行 rpmbuild...\n")
	cmd := exec.Command("rpmbuild", "-ba", specDest,
		"--define", fmt.Sprintf("_topdir %s", buildRoot),
		"--define", fmt.Sprintf("version %s", comp.Version),
		"--define", fmt.Sprintf("dist .%s", cfg.Dist),
		"--define", fmt.Sprintf("comp_name %s", comp.Name),
		"--define", fmt.Sprintf("pkg_name %s", comp.Name),
		"--target", cfg.Arch,
	)

	// 捕获输出用于调试，目前仅在报错时打印
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(output))
		return fmt.Errorf("rpmbuild 执行失败: %w", err)
	}

	// 5. 更新缓存
	// 查找在 RPMS 目录下生成的新 RPM
	builtRPMs, _ := filepath.Glob(filepath.Join(buildRoot, "RPMS", cfg.Arch, cachePattern))
	// 同时也检查 noarch 目录
	builtNoarch, _ := filepath.Glob(filepath.Join(buildRoot, "RPMS", "noarch", cachePattern))
	builtRPMs = append(builtRPMs, builtNoarch...)

	if len(builtRPMs) == 0 {
		fmt.Println(string(output))
		return fmt.Errorf("构建看起来成功了，但未找到符合模式 %s 的 RPM 产物", cachePattern)
	}

	for _, rpm := range builtRPMs {
		copyFile(rpm, filepath.Join(cacheDir, filepath.Base(rpm)))
	}
	fmt.Printf("   ✅ 构建成功，已缓存 %d 个 RPM\n", len(builtRPMs))

	return nil
}

func createTarball(tarPath, srcDir, prefix string) error {
	// 使用 'tar' 命令进行打包，模拟原脚本行为
	// -C parent_of_srcDir -czf tarPath basename_of_srcDir
	// 注意：我们假设 SPEC 文件期望的解压目录名与源码目录名一致
	// 如果不一致，可能需要更复杂的 tar 变换逻辑

	// We'll use the parent directory of SourceDir as the context
	parent := filepath.Dir(srcDir)
	base := filepath.Base(srcDir)

	cmd := exec.Command("tar", "-czf", tarPath, "-C", parent, base)
	// If the folder inside tar needs to be renamed to 'prefix', handling that with simple tar command is tricky
	// unless we use --transform.
	// For now, we assume SourceDir leaf name matches what SPEC expects.

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
