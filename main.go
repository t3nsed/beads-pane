package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	themeFlag := flag.String("theme", "light", "color theme: light or dark")
	flag.Parse()

	if _, err := exec.LookPath("bd"); err != nil {
		fmt.Fprintln(os.Stderr, "Error: 'bd' command not found.")
		fmt.Fprintln(os.Stderr, "Install beads: https://github.com/steveyegge/beads")
		os.Exit(1)
	}

	cfg, err := loadConfig()
	if err != nil {
		cfg, err = firstRunSetup()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Setup failed: %v\n", err)
			os.Exit(1)
		}
	}

	beadsDirs := scanForBeads(cfg.ScanRoots, cfg.MaxScanDepth)
	if len(beadsDirs) == 0 {
		fmt.Fprintln(os.Stderr, "No .beads directories found in configured scan roots.")
		fmt.Fprintln(os.Stderr, "Initialize a repo with 'bd init' or update scan roots in:")
		fmt.Fprintf(os.Stderr, "  %s\n", configPath())
		os.Exit(1)
	}

	fmt.Printf("Found %d beads repo(s). Starting dashboard...\n", len(beadsDirs))

	dashboard := NewDashboard(cfg, beadsDirs, *themeFlag)
	if err := dashboard.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
