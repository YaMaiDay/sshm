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
	listOnly := flag.Bool("list", false, "列出解析到的服务器后退出")
	probeHost := flag.String("probe", "", "采集指定服务器别名的监控信息后退出")
	remoteDirsHost := flag.String("remote-dirs", "", "列出指定服务器别名的远程常用目录后退出")
	configPath := flag.Bool("config-path", false, "显示应用配置文件路径后退出")
	showVersion := flag.Bool("version", false, "显示版本号后退出")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		tui.Fatal("无法获取用户目录", err)
	}

	hosts, err := config.LoadHosts(home)
	if err != nil {
		tui.Fatal("读取 SSH 配置失败", err)
	}

	if _, ok, err := config.LoadServerHosts(home); err != nil {
		tui.Fatal("读取服务器配置失败", err)
	} else if !ok {
		if err := config.MigrateServersFile(home, hosts, config.LoadPasswords(home)); err != nil {
			tui.Fatal("创建服务器配置失败", err)
		}
		hosts, err = config.LoadHosts(home)
		if err != nil {
			tui.Fatal("读取服务器配置失败", err)
		}
	}
	passwords := config.PasswordsFromHosts(hosts)
	if *configPath {
		fmt.Println(config.AppConfigPath(home))
		return
	}
	if *listOnly {
		for _, h := range hosts {
			password := "否"
			if h.HasPassword {
				password = "是"
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
		fmt.Printf("服务器：%s/%s\n", h.Category, h.Name)
		fmt.Printf("在线: %v\n", m.Online)
		fmt.Printf("系统: %s\n", m.OS)
		fmt.Printf("内核: %s\n", m.Kernel)
		fmt.Printf("架构: %s\n", m.Arch)
		fmt.Printf("CPU: %.0f%% %s %s\n", m.CPUPercent, cpuCoresText(m), m.CPUModel)
		fmt.Printf("内存: %.0f%% %s / %s\n", m.MemPercent(), bytesHuman(m.MemUsed), bytesHuman(m.MemTotal))
		fmt.Printf("Swap: %.0f%% %s / %s\n", m.SwapPercent(), bytesHuman(m.SwapUsed), bytesHuman(m.SwapTotal))
		fmt.Printf("磁盘: %.0f%% %s / %s\n", m.DiskPercent(), bytesHuman(m.DiskUsed), bytesHuman(m.DiskTotal))
		fmt.Printf("磁盘挂载: %s %s\n", m.DiskMountpoint, m.DiskFilesystem)
		fmt.Printf("inode: %.0f%% %s / %s\n", m.InodePercent(), countHuman(m.InodeUsed), countHuman(m.InodeTotal))
		if m.HealthTotal() > 0 {
			fmt.Printf("健康端口: %d/%d %s\n", m.HealthOK(), m.HealthTotal(), healthPortsText(m))
		}
		fmt.Printf("容器: %d/%d 运行，停止 %d，故障 %d\n", m.DockerRunning, dockerTotal(m), m.DockerStopped, m.DockerFailed)
		if len(m.DockerRunningNames) > 0 {
			fmt.Printf("运行容器: %s\n", strings.Join(m.DockerRunningNames, "、"))
		}
		if len(m.DockerStoppedNames) > 0 {
			fmt.Printf("停止容器: %s\n", strings.Join(m.DockerStoppedNames, "、"))
		}
		if len(m.DockerFailedNames) > 0 {
			fmt.Printf("故障容器: %s\n", strings.Join(m.DockerFailedNames, "、"))
		}
		fmt.Printf("负载: %s %s %s\n", m.Load1, m.Load5, m.Load15)
		fmt.Printf("运行: %s\n", m.Uptime)
		if m.Error != "" {
			fmt.Printf("错误: %s\n", m.Error)
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
		tui.Fatal("运行 TUI 失败", err)
	}
}

func findHost(hosts []host.Host, query string) (host.Host, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return host.Host{}, fmt.Errorf("服务器名称不能为空")
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
		return host.Host{}, fmt.Errorf("没有找到指定服务器：%s", query)
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
		return host.Host{}, fmt.Errorf("服务器名称不唯一，请使用 分类/名称：%s", strings.Join(options, "、"))
	}
	return host.Host{}, fmt.Errorf("没有找到指定服务器：%s", query)
}

func cpuCoresText(metrics monitor.Metrics) string {
	if metrics.CPUCores <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d核", metrics.CPUCores)
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
		status := "失败"
		if port.Healthy {
			status = "正常"
		}
		parts = append(parts, fmt.Sprintf("%d%s", port.Port, status))
	}
	return strings.Join(parts, " ")
}
