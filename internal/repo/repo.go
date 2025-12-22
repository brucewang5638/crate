package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"crate/internal/config"
)

func Generate(cfg *config.Config, releaseDir string) error {
	repoDir := filepath.Join(releaseDir, "hk-repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return err
	}

	fmt.Println("\n📦 Downloading System Dependencies...")
	// 1. Repotrack (Downloads dependencies)
	// Note: repotrack might not be available or slow. We assume it's installed as per requirement.
	// We iterate over SystemDeps from config
	for _, pkg := range cfg.SystemDeps {
		fmt.Printf("   -> %s\n", pkg)
		cmd := exec.Command("repotrack", "-p", repoDir, pkg)
		if err := cmd.Run(); err != nil {
			// Log but don't fail immediately? Or strict mode?
			// For now, strict mode off to avoid breaking if one lib fails (e.g. already local)
			fmt.Printf("      ⚠️ Warning: repotrack failed for %s: %v\n", pkg, err)
		}
	}

	// 2. Createrepo
	fmt.Println("\n📂 Generating Repo Metadata (createrepo)...")
	cmd := exec.Command("createrepo", repoDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println(string(out))
		return fmt.Errorf("createrepo failed: %w", err)
	}

	// 3. Setup Script
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
REPO_PATH="${CUR_DIR}/hk-repo"

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
