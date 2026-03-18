package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	defaultPollIntervalSec = 30
	defaultMaxScanDepth    = 6
)

// Config persisted to disk between runs.
type Config struct {
	ScanRoots       []string `json:"scan_roots"`
	PollIntervalSec int      `json:"poll_interval_seconds"`
	MaxScanDepth    int      `json:"max_scan_depth"`
}

func configDir() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "beads-pane")
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "beads-pane")
	}
	return filepath.Join(home, ".config", "beads-pane")
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

func loadConfig() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.PollIntervalSec <= 0 {
		cfg.PollIntervalSec = defaultPollIntervalSec
	}
	if cfg.MaxScanDepth <= 0 {
		cfg.MaxScanDepth = defaultMaxScanDepth
	}
	return &cfg, nil
}

func saveConfig(cfg *Config) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0644)
}

func firstRunSetup() (*Config, error) {
	home, _ := os.UserHomeDir()
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("  ┌──────────────────────────────────────┐")
	fmt.Println("  │  beads-pane  ◆  First Run Setup      │")
	fmt.Println("  └──────────────────────────────────────┘")
	fmt.Println()
	fmt.Println("  Where should I scan for repos with .beads directories?")
	fmt.Println("  Enter path(s) separated by commas.")
	fmt.Printf("  Press Enter to use default [%s]:\n\n", home)
	fmt.Print("  > ")

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	var roots []string
	if input == "" {
		roots = []string{home}
	} else {
		for _, r := range strings.Split(input, ",") {
			r = strings.TrimSpace(r)
			if strings.HasPrefix(r, "~/") {
				r = filepath.Join(home, r[2:])
			} else if r == "~" {
				r = home
			}
			if r != "" {
				roots = append(roots, r)
			}
		}
	}
	if len(roots) == 0 {
		roots = []string{home}
	}

	cfg := &Config{
		ScanRoots:       roots,
		PollIntervalSec: defaultPollIntervalSec,
		MaxScanDepth:    defaultMaxScanDepth,
	}

	if err := saveConfig(cfg); err != nil {
		return nil, fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("\n  Config saved to %s\n", configPath())
	fmt.Printf("  Scanning %v (depth %d) ...\n\n", roots, cfg.MaxScanDepth)
	return cfg, nil
}
