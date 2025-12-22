package main

import (
	"flag"
	"fmt"
	"os"

	"crate/internal/config"
	"crate/internal/builder"
)

func main() {
	configFile := flag.String("config", "config.yaml", "Path to configuration file")
	buildTarget := flag.String("build", "", "Component or Group to build (e.g. 'redis', 'components')")
	// release := flag.Bool("release", false, "Generate final release distribution")
	arch := flag.String("arch", "", "Override target architecture")
	dist := flag.String("dist", "", "Override distribution (e.g. el7)")

	flag.Parse()

	// 1. Load Config
	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Printf("❌ Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Override Config with Flags
	if *arch != "" {
		cfg.Arch = *arch
	}
	if *dist != "" {
		cfg.Dist = *dist
	}

	fmt.Printf("🌟 Crate Builder v0.1 | Arch: %s | Dist: %s\n", cfg.Arch, cfg.Dist)

	// 2. Execute Build
	if *buildTarget != "" {
		if err := builder.Build(cfg, *buildTarget); err != nil {
			fmt.Printf("❌ Build failed: %v\n", err)
			os.Exit(1)
		}
	} else {
		flag.Usage()
	}
}
