package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/fsselect"
	"github.com/YaMaiDay/sshm/internal/host"
	"github.com/YaMaiDay/sshm/internal/monitor"
	"github.com/YaMaiDay/sshm/internal/tui"
)

var version = "dev"

func main() {
	listOnly := flag.Bool("list", false, "list configured servers and exit")
	probeHost := flag.String("probe", "", "collect monitoring data for a server alias and exit")
	remoteDirsHost := flag.String("remote-dirs", "", "list common remote directories for a server alias and exit")
	configPath := flag.Bool("config-path", false, "print the app settings file path and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		tui.Fatal("cannot get user home directory", err)
	}

	hosts, err := config.LoadHosts(home)
	if err != nil {
		tui.Fatal("failed to read SSH config", err)
	}

	if _, ok, err := config.LoadServerHosts(home); err != nil {
		tui.Fatal("failed to read server config", err)
	} else if !ok {
		if err := config.MigrateServersFile(home, hosts, config.LoadPasswords(home)); err != nil {
			tui.Fatal("failed to create server config", err)
		}
		hosts, err = config.LoadHosts(home)
		if err != nil {
			tui.Fatal("failed to read server config", err)
		}
	}
	passwords := config.PasswordsFromHosts(hosts)
	if *configPath {
		fmt.Println(config.AppConfigPath(home))
		return
	}
	if *listOnly {
		for _, h := range hosts {
			password := "no"
			if h.HasPassword {
				password = "yes"
			}
			fmt.Printf("%-16s %-16s %-10s %-8s %-8s\n", h.Name, h.Address(), h.User, h.Category, password)
		}
		return
	}
	if *probeHost != "" {
		collector := monitor.NewCollector(passwords)
		appConfig := config.LoadAppConfig(home)
		collector.Timeout = appConfig.CommandDuration()
		collector.ConnectTimeout = appConfig.ConnectDuration()
		h, err := findHost(hosts, *probeHost)
		if err != nil {
			tui.Fatal(err.Error(), nil)
		}
		m := collector.Collect(context.Background(), h)
		fmt.Printf("Server: %s/%s\n", h.Category, h.Name)
		fmt.Printf("Online: %v\n", m.Online)
		fmt.Printf("OS: %s\n", m.OS)
		fmt.Printf("Kernel: %s\n", m.Kernel)
		fmt.Printf("Arch: %s\n", m.Arch)
		fmt.Printf("CPU: %.0f%% %s %s\n", m.CPUPercent, cpuCoresText(m), m.CPUModel)
		fmt.Printf("Memory: %.0f%% %s / %s\n", m.MemPercent(), bytesHuman(m.MemUsed), bytesHuman(m.MemTotal))
		fmt.Printf("Swap: %.0f%% %s / %s\n", m.SwapPercent(), bytesHuman(m.SwapUsed), bytesHuman(m.SwapTotal))
		fmt.Printf("Disk: %.0f%% %s / %s\n", m.DiskPercent(), bytesHuman(m.DiskUsed), bytesHuman(m.DiskTotal))
		fmt.Printf("Disk mount: %s %s\n", m.DiskMountpoint, m.DiskFilesystem)
		fmt.Printf("inode: %.0f%% %s / %s\n", m.InodePercent(), countHuman(m.InodeUsed), countHuman(m.InodeTotal))
		if m.HealthTotal() > 0 {
			fmt.Printf("Health ports: %d/%d %s\n", m.HealthOK(), m.HealthTotal(), healthPortsText(m))
		}
		fmt.Printf("Containers: %d/%d running, stopped %d, failed %d\n", m.DockerRunning, dockerTotal(m), m.DockerStopped, m.DockerFailed)
		if len(m.DockerRunningNames) > 0 {
			fmt.Printf("Running containers: %s\n", strings.Join(m.DockerRunningNames, ", "))
		}
		if len(m.DockerStoppedNames) > 0 {
			fmt.Printf("Stopped containers: %s\n", strings.Join(m.DockerStoppedNames, ", "))
		}
		if len(m.DockerFailedNames) > 0 {
			fmt.Printf("Failed containers: %s\n", strings.Join(m.DockerFailedNames, ", "))
		}
		fmt.Printf("Load: %s %s %s\n", m.Load1, m.Load5, m.Load15)
		fmt.Printf("Uptime: %s\n", m.Uptime)
		if m.Error != "" {
			fmt.Printf("Error: %s\n", m.Error)
		}
		return
	}
	if *remoteDirsHost != "" {
		h, err := findHost(hosts, *remoteDirsHost)
		if err != nil {
			tui.Fatal(err.Error(), nil)
		}
		for _, dir := range fsselect.RemoteDirs(h) {
			fmt.Println(dir)
		}
		return
	}

	if err := tui.Run(hosts, passwords); err != nil {
		tui.Fatal("failed to run TUI", err)
	}
}

func findHost(hosts []host.Host, query string) (host.Host, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return host.Host{}, fmt.Errorf("server name cannot be empty")
	}
	if strings.Contains(query, "/") {
		category, name, _ := strings.Cut(query, "/")
		category = strings.TrimSpace(category)
		name = strings.TrimSpace(name)
		for _, h := range hosts {
			if h.Category == category && h.Name == name {
				return h, nil
			}
		}
		return host.Host{}, fmt.Errorf("server not found: %s", query)
	}
	matches := make([]host.Host, 0, 2)
	for _, h := range hosts {
		if h.Name == query {
			matches = append(matches, h)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		options := make([]string, 0, len(matches))
		for _, h := range matches {
			options = append(options, h.Category+"/"+h.Name)
		}
		return host.Host{}, fmt.Errorf("server name is ambiguous; use category/name: %s", strings.Join(options, ", "))
	}
	return host.Host{}, fmt.Errorf("server not found: %s", query)
}

func cpuCoresText(metrics monitor.Metrics) string {
	if metrics.CPUCores <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d cores", metrics.CPUCores)
}

func dockerTotal(metrics monitor.Metrics) int {
	if metrics.DockerTotal > 0 {
		return metrics.DockerTotal
	}
	return metrics.DockerRunning
}

func bytesHuman(value uint64) string {
	if value == 0 {
		return "-"
	}
	units := []string{"B", "K", "M", "G", "T"}
	f := float64(value)
	unit := 0
	for f >= 1024 && unit < len(units)-1 {
		f /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%.0f%s", f, units[unit])
	}
	return fmt.Sprintf("%.1f%s", f, units[unit])
}

func countHuman(value uint64) string {
	if value == 0 {
		return "-"
	}
	units := []string{"", "K", "M", "B"}
	f := float64(value)
	unit := 0
	for f >= 1000 && unit < len(units)-1 {
		f /= 1000
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%.0f", f)
	}
	return fmt.Sprintf("%.1f%s", f, units[unit])
}

func healthPortsText(metrics monitor.Metrics) string {
	parts := make([]string, 0, len(metrics.HealthPorts))
	for _, port := range metrics.HealthPorts {
		status := "failed"
		if port.Healthy {
			status = "ok"
		}
		parts = append(parts, fmt.Sprintf("%d:%s", port.Port, status))
	}
	return strings.Join(parts, " ")
}
