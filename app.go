package main

import (
	"context"
	"fmt"
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

