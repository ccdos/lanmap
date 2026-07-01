package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// GetDefaultSubnet returns the most likely local subnet
func (a *App) GetDefaultSubnet() string {
	return GetDefaultSubnet()
}

// ScanSubnet starts the subnet scanning process and emits events
func (a *App) ScanSubnet(subnet string, method string) {
	go ScanSubnet(a.ctx, subnet, method)
}

// ProbeIP returns detailed information about an IP address synchronously
func (a *App) ProbeIP(ip string) ProbeResult {
	return ProbeIP(ip)
}

func getHistoryFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".lanmap_history.json"
	}
	return filepath.Join(home, ".lanmap_history.json")
}

// LoadHistory loads the subnet scan history from disk
func (a *App) LoadHistory() []string {
	file, err := os.ReadFile(getHistoryFilePath())
	if err != nil {
		return []string{}
	}
	var history []string
	if err := json.Unmarshal(file, &history); err != nil {
		return []string{}
	}
	return history
}

// SaveHistory saves the subnet scan history to disk
func (a *App) SaveHistory(history []string) {
	data, err := json.MarshalIndent(history, "", "  ")
	if err == nil {
		_ = os.WriteFile(getHistoryFilePath(), data, 0644)
	}
}

