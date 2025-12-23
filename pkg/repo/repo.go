package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"crate/pkg/config"
)

// Generate 生成离线 YUM 仓库
func Generate(cfg *config.Config, releaseDir string) error {
	repoDir := filepath.Join(releaseDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return err
	}

	// 检测是否为 Kylin 系统
	dist := strings.ToLower(cfg.Dist)
	isKylin := strings.Contains(dist, "kylin") || strings.Contains(dist, "v10")
	// 更鲁棒的判断
	// if strings.Contains(strings.ToLower(cfg.Dist), "kylin") ... (需要引入 strings 包)

	var err error
	if isKylin {
		err = generateKylin(cfg, repoDir)
	} else {
		err = generateEl7(cfg, repoDir)
	}

	if err != nil {
		return err
	}

	// 3. 生成安装脚本
	return createSetupScript(releaseDir)
}

// generateEl7 使用 repotrack (RHEL/CentOS 7)
func generateEl7(cfg *config.Config, repoDir string) error {
	depsCache, err := ensureDepsCache(cfg)
	if err != nil {
		return fmt.Errorf("无法创建依赖缓存: %w", err)
	}

	fmt.Printf("\n📦 [EL7] 正在下载系统依赖 (缓存目录: %s)...\n", depsCache)

	for _, pkg := range cfg.SystemDeps {
		fmt.Printf("   -> %s\n", pkg)
		// 下载到缓存目录
		cmd := exec.Command("repotrack", "-p", depsCache, pkg)
		if err := cmd.Run(); err != nil {
			fmt.Printf("      ⚠️ 警告: repotrack 下载 %s 失败: %v\n", pkg, err)
		}
	}

	// 从缓存同步到当前的 repo 目录
	if err := syncCacheToRepo(depsCache, repoDir); err != nil {
		return err
	}

	// 生成元数据
	return runCreaterepo(repoDir, "createrepo")
}

// generateKylin 使用 dnf (Kylin V10+)
func generateKylin(cfg *config.Config, repoDir string) error {
	depsCache, err := ensureDepsCache(cfg)
	if err != nil {
		return fmt.Errorf("无法创建依赖缓存: %w", err)
	}

	fmt.Printf("\n🐉 [Kylin] 正在下载系统依赖 (缓存目录: %s)...\n", depsCache)

	// 1. 检查并安装 createrepo_c
	if _, err := exec.LookPath("createrepo_c"); err != nil {
		fmt.Println("   🔧 检测到缺少 createrepo_c，尝试安装...")
		// yum install -y dnf-utils createrepo_c
		installCmd := exec.Command("yum", "install", "-y", "dnf-utils", "createrepo_c")
		if out, err := installCmd.CombinedOutput(); err != nil {
			fmt.Printf("      ⚠️ 安装辅助工具可能有误 (忽略): %s\n", string(out))
		}
	}

	// 2. 准备依赖列表 (追加 createrepo_c)
	deps := append(cfg.SystemDeps, "createrepo_c")

	// 3. 批量下载
	// 下载到缓存目录
	args := []string{"download", "--resolve", "--alldeps", "--destdir", depsCache}
	args = append(args, deps...)

	fmt.Printf("   🚀 执行 DNF 批量下载 (%d 个包)...\n", len(deps))
	cmd := exec.Command("dnf", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dnf download 失败: %w", err)
	}

	// 从缓存同步到当前的 repo 目录
	if err := syncCacheToRepo(depsCache, repoDir); err != nil {
		return err
	}

	// 生成元数据 (使用 createrepo_c)
	return runCreaterepo(repoDir, "createrepo_c")
}

func runCreaterepo(dir string, toolName string) error {
	fmt.Printf("\n📂 生成仓库元数据 (%s)...\n", toolName)
	cmd := exec.Command(toolName, dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println(string(out))
		return fmt.Errorf("%s 执行失败: %w", toolName, err)
	}
	return nil
}

func createSetupScript(releaseDir string) error {
	scriptPath := filepath.Join(releaseDir, "setup_repo.sh")
	content := `#!/bin/bash
set -e
CUR_DIR="$(cd "$(dirname "$0")"; pwd)"
REPO_PATH="${CUR_DIR}/repo"

echo "Configuring local repo..."
cat > /etc/yum.repos.d/local.repo <<EOR
[local]
name=Offline_Repository
baseurl=file://$REPO_PATH
enabled=1
gpgcheck=0
EOR

yum clean all
yum makecache --disablerepo=* --enablerepo=local

echo "Done! Please setup your services manually."
`
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
		return err
	}
	// ... existing code ...
	return nil
}

// 辅助函数: 确保依赖缓存目录存在
func ensureDepsCache(cfg *config.Config) (string, error) {
	cacheDir := cfg.CacheDir
	if strings.HasPrefix(cacheDir, "~/") {
		dirname, _ := os.UserHomeDir()
		cacheDir = filepath.Join(dirname, cacheDir[2:])
	}
	depsDir := filepath.Join(cacheDir, "deps", cfg.Dist) // 按发行版隔离缓存
	if err := os.MkdirAll(depsDir, 0755); err != nil {
		return "", err
	}
	return depsDir, nil
}

// 辅助函数: 将文件从缓存同步到 Repo 目录
func syncCacheToRepo(srcDir, dstDir string) error {
	fmt.Printf("   ♻️  正在从缓存同步 RPM 包...\n")
	files, err := filepath.Glob(filepath.Join(srcDir, "*.rpm"))
	if err != nil {
		return err
	}

	for _, f := range files {
		baseName := filepath.Base(f)
		dst := filepath.Join(dstDir, baseName)
		// 如果目标不存在，或者大小不同，则复制 (简单同步逻辑)
		// 这里为了简单，直接覆盖
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Printf("      ⚠️ 读取缓存文件失败 %s: %v\n", baseName, err)
			continue
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return fmt.Errorf("写入文件失败: %w", err)
		}
	}
	return nil
}
