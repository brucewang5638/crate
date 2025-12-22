package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"crate/internal/config"
)

// Generate 生成离线 YUM 仓库
func Generate(cfg *config.Config, releaseDir string) error {
	repoDir := filepath.Join(releaseDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return err
	}

	fmt.Println("\n📦 正在下载系统依赖...")
	// 1. Repotrack (下载依赖)
	// 注意: repotrack 可能不存在或很慢。我们假设已按要求安装。
	for _, pkg := range cfg.SystemDeps {
		fmt.Printf("   -> %s\n", pkg)
		cmd := exec.Command("repotrack", "-p", repoDir, pkg)
		if err := cmd.Run(); err != nil {
			// 是否需要严格失败？目前仅警告，防止因为本地已存在而导致整体流程挂掉
			fmt.Printf("      ⚠️ 警告: repotrack 下载 %s 失败: %v\n", pkg, err)
		}
	}

	// 2. Createrepo
	fmt.Println("\n📂 生成仓库元数据 (createrepo)...")
	cmd := exec.Command("createrepo", repoDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println(string(out))
		return fmt.Errorf("createrepo 执行失败: %w", err)
	}

	// 3. 生成安装脚本
	if err := createSetupScript(releaseDir); err != nil {
		return err
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
