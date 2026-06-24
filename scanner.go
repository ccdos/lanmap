package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type ScanResult struct {
	IP     string `json:"ip"`
	Status string `json:"status"` // "active" or "inactive"
	MAC    string `json:"mac"`
}

// GetDefaultSubnet returns the most likely local subnet (e.g. 192.168.1.0/24)
func GetDefaultSubnet() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "192.168.1.0/24"
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					network := ipnet.IP.Mask(ipnet.Mask)
					ones, _ := ipnet.Mask.Size()
					return fmt.Sprintf("%s/%d", network.String(), ones)
				}
			}
		}
	}
	return "192.168.1.0/24"
}

// getArpCache returns a map of IP to MAC address
func getArpCache() map[string]string {
	cache := make(map[string]string)
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("arp", "-a")
	} else {
		cmd = exec.Command("arp", "-an")
	}

	out, err := cmd.Output()
	if err != nil {
		return cache
	}

	ipRegex := regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)
	macRegex := regexp.MustCompile(`([0-9A-Fa-f]{2}[:-][0-9A-Fa-f]{2}[:-][0-9A-Fa-f]{2}[:-][0-9A-Fa-f]{2}[:-][0-9A-Fa-f]{2}[:-][0-9A-Fa-f]{2})`)

	lines := bytes.Split(out, []byte("\n"))
	for _, line := range lines {
		s := string(line)
		ipMatch := ipRegex.FindStringSubmatch(s)
		macMatch := macRegex.FindStringSubmatch(s)
		if len(ipMatch) > 1 && len(macMatch) > 1 {
			cache[ipMatch[1]] = strings.ReplaceAll(macMatch[1], "-", ":")
		}
	}
	return cache
}

func getAllIPsInSubnet(subnetCIDR string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(subnetCIDR)
	if err != nil {
		return nil, err
	}

	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}
	if len(ips) > 2 {
		return ips[1 : len(ips)-1], nil
	}
	return ips, nil
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// ScanSubnet performs the scanning and emits events
func ScanSubnet(ctx context.Context, subnet string, method string) {
	ips, err := getAllIPsInSubnet(subnet)
	if err != nil {
		wailsRuntime.EventsEmit(ctx, "scan_error", err.Error())
		return
	}

	wailsRuntime.EventsEmit(ctx, "scan_start", len(ips))

	var wg sync.WaitGroup
	var mu sync.Mutex
	resultsMap := make(map[string]ScanResult)

	sem := make(chan struct{}, 150)

	for _, ip := range ips {
		wg.Add(1)
		sem <- struct{}{}
		go func(targetIP string) {
			defer wg.Done()
			defer func() { <-sem }()

			isActive := false

			if method == "ARP" {
				conn, err := net.DialTimeout("udp", targetIP+":53", 150*time.Millisecond)
				if err == nil {
					conn.Close()
				}
			} else if method == "ICMP" {
				pinger, err := probing.NewPinger(targetIP)
				if err == nil {
					pinger.Count = 1
					pinger.Timeout = 500 * time.Millisecond
					pinger.SetPrivileged(false)
					if runtime.GOOS == "windows" {
						pinger.SetPrivileged(true) // Windows requires privileged ping for go-ping
					}
					err = pinger.Run()
					if err == nil && pinger.Statistics().PacketsRecv > 0 {
						isActive = true
					}
				}
			} else if method == "TCP" {
				ports := []string{"80", "443", "135", "445", "3389", "22"}
				for _, port := range ports {
					conn, err := net.DialTimeout("tcp", targetIP+":"+port, 200*time.Millisecond)
					if err == nil {
						conn.Close()
						isActive = true
						break
					}
				}
			}

			if isActive && method != "ARP" {
				mu.Lock()
				resultsMap[targetIP] = ScanResult{IP: targetIP, Status: "active"}
				mu.Unlock()
			}
		}(ip)
	}

	wg.Wait()
	
	// Wait a tiny bit for ARP cache to populate
	if method == "ARP" {
		time.Sleep(200 * time.Millisecond)
	}

	arpCache := getArpCache()

	for _, ip := range ips {
		res := ScanResult{IP: ip, Status: "inactive"}
		
		if method == "ARP" {
			if mac, ok := arpCache[ip]; ok && mac != "ff:ff:ff:ff:ff:ff" {
				res.Status = "active"
				res.MAC = mac
			}
		} else {
			mu.Lock()
			if r, ok := resultsMap[ip]; ok {
				res.Status = "active"
				if mac, ok := arpCache[ip]; ok {
					res.MAC = mac
				} else {
					res.MAC = r.MAC
				}
			}
			mu.Unlock()
		}
		wailsRuntime.EventsEmit(ctx, "scan_result", res)
	}

	wailsRuntime.EventsEmit(ctx, "scan_complete", true)
}

type ProbeResult struct {
	IP       string   `json:"ip"`
	Hostname string   `json:"hostname"`
	Latency  string   `json:"latency"`
	OSGuess  string   `json:"os_guess"`
	Ports    []string `json:"ports"`
	WebTitle string   `json:"web_title"`
}

func getWebTitle(ip string, port string) string {
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	scheme := "http"
	if port == "443" || port == "5001" {
		scheme = "https"
	}
	resp, err := client.Get(fmt.Sprintf("%s://%s:%s", scheme, ip, port))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*10)) // Read up to 10KB
	if err != nil {
		return ""
	}

	re := regexp.MustCompile(`(?i)<title>(.*?)</title>`)
	match := re.FindStringSubmatch(string(body))
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

// ProbeIP performs a deep scan of an IP and returns the info synchronously
func ProbeIP(ip string) ProbeResult {
	res := ProbeResult{
		IP:      ip,
		Ports:   []string{},
		OSGuess: "Unknown",
	}

	// 1. Hostname
	names, _ := net.LookupAddr(ip)
	if len(names) > 0 {
		res.Hostname = strings.TrimRight(names[0], ".")
	} else {
		res.Hostname = "N/A"
	}

	// 2. Latency
	pinger, err := probing.NewPinger(ip)
	if err == nil {
		pinger.Count = 1
		pinger.Timeout = 500 * time.Millisecond
		pinger.SetPrivileged(false)
		if runtime.GOOS == "windows" {
			pinger.SetPrivileged(true)
		}
		if pinger.Run() == nil && pinger.Statistics().PacketsRecv > 0 {
			res.Latency = pinger.Statistics().AvgRtt.String()
		} else {
			res.Latency = "Timeout"
		}
	} else {
		res.Latency = "N/A"
	}

	// 3. Port Scan
	var wg sync.WaitGroup
	var mu sync.Mutex
	portsToScan := map[string]string{
		"22":   "SSH",
		"80":   "HTTP",
		"443":  "HTTPS",
		"135":  "RPC",
		"445":  "SMB",
		"3389": "RDP",
		"5000": "UPnP/NAS",
		"5001": "NAS(SSL)",
		"8080": "HTTP-Alt",
	}

	for port, name := range portsToScan {
		wg.Add(1)
		go func(p, n string) {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", ip+":"+p, 500*time.Millisecond)
			if err == nil {
				conn.Close()
				mu.Lock()
				res.Ports = append(res.Ports, p+" ("+n+")")
				mu.Unlock()
			}
		}(port, name)
	}
	wg.Wait()

	// 4. Web Title
	hasPort := func(p string) bool {
		for _, v := range res.Ports {
			if strings.HasPrefix(v, p+" ") {
				return true
			}
		}
		return false
	}

	if hasPort("80") {
		res.WebTitle = getWebTitle(ip, "80")
	}
	if res.WebTitle == "" && hasPort("8080") {
		res.WebTitle = getWebTitle(ip, "8080")
	}
	if res.WebTitle == "" && hasPort("443") {
		res.WebTitle = getWebTitle(ip, "443")
	}
	if res.WebTitle == "" && hasPort("5000") {
		res.WebTitle = getWebTitle(ip, "5000")
	}

	// 5. OS Guessing based on ports & web title
	lowerTitle := strings.ToLower(res.WebTitle)
	if strings.Contains(lowerTitle, "qnap") || strings.Contains(lowerTitle, "synology") || strings.Contains(lowerTitle, "truenas") || strings.Contains(lowerTitle, "nas") || hasPort("5000") || hasPort("5001") {
		res.OSGuess = "Linux / NAS"
	} else if strings.Contains(lowerTitle, "router") || strings.Contains(lowerTitle, "openwrt") || strings.Contains(lowerTitle, "tp-link") {
		res.OSGuess = "Router / Network Device"
	} else if hasPort("3389") || hasPort("135") {
		res.OSGuess = "Windows"
	} else if hasPort("445") {
		if hasPort("22") {
			res.OSGuess = "Linux / NAS (Samba+SSH)"
		} else {
			res.OSGuess = "Windows"
		}
	} else if hasPort("22") {
		res.OSGuess = "Linux / macOS"
	} else if len(res.Ports) > 0 {
		res.OSGuess = "Other / IoT Device"
	}

	return res
}
