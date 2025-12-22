package builder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"crate/internal/config"
)

func Build(cfg *config.Config, target string) error {
	targets, err := cfg.FindComponents(target)
	if err != nil {
		return err
	}

	fmt.Printf("🎯 Building %d components for target '%s'\n", len(targets), target)

	for _, comp := range targets {
		if err := buildOne(cfg, comp); err != nil {
			return fmt.Errorf("failed to build %s: %w", comp.Name, err)
		}
	}

	return nil
}

// BuildOne handles the lifecycle of a single component build
func buildOne(cfg *config.Config, comp config.Component) error {
	fmt.Printf("\n🔧 [Building] %s (v%s)...\n", comp.Name, comp.Version)

	cacheDir := expandPath(cfg.CacheDir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache dir: %w", err)
	}

	// 0. Check Cache (Simple pattern matching)
	// Pattern assumption: hk-component-name-version*.rpm or similar
	// For simplicity, we search for *name-version*.rpm
	cachePattern := fmt.Sprintf("*%s-%s*.rpm", comp.Name, comp.Version)
	matches, _ := filepath.Glob(filepath.Join(cacheDir, cachePattern))
	if len(matches) > 0 {
		fmt.Printf("   📦 Cache hit: found %d RPMs for %s\n", len(matches), comp.Name)
		// Copy to RPMS dir
		rpmDir := filepath.Join(expandPath(cfg.BuildRoot), "RPMS")
		os.MkdirAll(rpmDir, 0755)
		for _, m := range matches {
			dest := filepath.Join(rpmDir, filepath.Base(m))
			copyFile(m, dest)
		}
		return nil
	}

	// 1. Prepare Environment
	buildRoot := expandPath(cfg.BuildRoot)
	rpmDirs := []string{"BUILD", "BUILDROOT", "RPMS", "SOURCES", "SPECS", "SRPMS"}
	for _, d := range rpmDirs {
		if err := os.MkdirAll(filepath.Join(buildRoot, d), 0755); err != nil {
			return fmt.Errorf("failed to create dir %s: %w", d, err)
		}
	}

	// 2. Prepare Source (Create Tarball)
	if comp.SourceDir != "" {
		tarName := fmt.Sprintf("%s.tar.gz", comp.Name)
		tarPath := filepath.Join(buildRoot, "SOURCES", tarName)

		fmt.Printf("   📦 Packing source: %s -> %s\n", comp.SourceDir, tarName)
		// Assuming createTarball handles errors correctly
		if err := createTarball(tarPath, comp.SourceDir, comp.Name); err != nil {
			return fmt.Errorf("failed to pack source: %w", err)
		}
	}

	// 3. Prepare SPEC File
	specSrc := comp.Spec
	if specSrc == "" {
		return fmt.Errorf("spec file not defined for %s", comp.Name)
	}
	specDest := filepath.Join(buildRoot, "SPECS", filepath.Base(specSrc))
	if err := copyFile(specSrc, specDest); err != nil {
		return fmt.Errorf("failed to copy spec: %w", err)
	}

	// 4. Run rpmbuild
	fmt.Printf("   🔨 Running rpmbuild...\n")
	cmd := exec.Command("rpmbuild", "-ba", specDest,
		"--define", fmt.Sprintf("_topdir %s", buildRoot),
		"--define", fmt.Sprintf("version %s", comp.Version),
		"--define", fmt.Sprintf("dist .%s", cfg.Dist),
		"--define", fmt.Sprintf("comp_name %s", comp.Name),
		"--define", fmt.Sprintf("pkg_name %s", comp.Name),
		"--target", cfg.Arch,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(output))
		return fmt.Errorf("rpmbuild execution failed: %w", err)
	}

	// 5. Update Cache
	// Find generated RPMs in RPMS dir and copy to cache
	builtRPMs, _ := filepath.Glob(filepath.Join(buildRoot, "RPMS", cfg.Arch, cachePattern))
	// Also check noarch
	builtNoarch, _ := filepath.Glob(filepath.Join(buildRoot, "RPMS", "noarch", cachePattern))
	builtRPMs = append(builtRPMs, builtNoarch...)

	if len(builtRPMs) == 0 {
		fmt.Println(string(output))
		return fmt.Errorf("build success but no RPMs found matching %s", cachePattern)
	}

	for _, rpm := range builtRPMs {
		copyFile(rpm, filepath.Join(cacheDir, filepath.Base(rpm)))
	}
	fmt.Printf("   ✅ Build success, cached %d RPMs\n", len(builtRPMs))

	return nil
}

func createTarball(tarPath, srcDir, prefix string) error {
	// Uses 'tar' command for simplicity and speed equivalent to original scripts
	// -C parent_of_srcDir -czf tarPath basename_of_srcDir
	// Note: The original scripts often tarred specific folders.
	// The config uses absolute paths. We need to be careful about what directory structure the SPEC expects.
	// Assumption: SPEC expects the tarball to unzip into a top-level directory named `prefix` (or uses %setup -n)

	// We'll use the parent directory of SourceDir as the context
	parent := filepath.Dir(srcDir)
	base := filepath.Base(srcDir)

	cmd := exec.Command("tar", "-czf", tarPath, "-C", parent, base)
	// If the folder inside tar needs to be renamed to 'prefix', handling that with simple tar command is tricky
	// unless we use --transform.
	// For now, we assume SourceDir leaf name matches what SPEC expects.

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar failed: %s (%w)", string(out), err)
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
