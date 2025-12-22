package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"crate/internal/config"
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
	fmt.Println("\n📦 [EL7] 正在下载系统依赖 (repotrack)...")
	for _, pkg := range cfg.SystemDeps {
		fmt.Printf("   -> %s\n", pkg)
		cmd := exec.Command("repotrack", "-p", repoDir, pkg)
		if err := cmd.Run(); err != nil {
			fmt.Printf("      ⚠️ 警告: repotrack 下载 %s 失败: %v\n", pkg, err)
		}
	}
	// 生成元数据
	return runCreaterepo(repoDir, "createrepo")
}

// generateKylin 使用 dnf (Kylin V10+)
func generateKylin(cfg *config.Config, repoDir string) error {
	fmt.Println("\n� [Kylin] 正在下载系统依赖 (dnf)...")

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
	// dnf download --resolve --alldeps --destdir="$REPO_DIR" "${DEPS[@]}"
	args := []string{"download", "--resolve", "--alldeps", "--destdir", repoDir}
	args = append(args, deps...)

	fmt.Printf("   🚀 执行 DNF 批量下载 (%d 个包)...\n", len(deps))
	cmd := exec.Command("dnf", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dnf download 失败: %w", err)
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
cat > /etc/yum.repos.d/hk-local.repo <<EOR
[hk-local]
name=HK_Offline_Repository
baseurl=file://$REPO_PATH
enabled=1
gpgcheck=0
EOR

yum clean all
yum makecache --disablerepo=* --enablerepo=hk-local

echo "Installing services..."
yum install -y hk-service-smzjg --disablerepo=* --enablerepo=hk-local

echo "Done!"
`
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
		return err
	}
	return nil
}
