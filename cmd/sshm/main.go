package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/YaMaiDay/sshm/internal/config"
	"github.com/YaMaiDay/sshm/internal/fsselect"
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
		for _, h := range hosts {
			if h.Name == *probeHost {
				m := collector.Collect(context.Background(), h)
				fmt.Printf("服务器：%s\n", h.Name)
				fmt.Printf("在线: %v\n", m.Online)
				fmt.Printf("系统: %s\n", m.OS)
				fmt.Printf("CPU: %.0f%%\n", m.CPUPercent)
				fmt.Printf("内存: %.0f%%\n", m.MemPercent())
				fmt.Printf("磁盘: %.0f%%\n", m.DiskPercent())
				fmt.Printf("负载: %s %s %s\n", m.Load1, m.Load5, m.Load15)
				fmt.Printf("运行: %s\n", m.Uptime)
				if m.Error != "" {
					fmt.Printf("错误: %s\n", m.Error)
				}
				return
			}
		}
		tui.Fatal("没有找到指定服务器："+*probeHost, nil)
	}
	if *remoteDirsHost != "" {
		for _, h := range hosts {
			if h.Name == *remoteDirsHost {
				for _, dir := range fsselect.RemoteDirs(h) {
					fmt.Println(dir)
				}
				return
			}
		}
		tui.Fatal("没有找到指定服务器："+*remoteDirsHost, nil)
		return
	}

	if err := tui.Run(hosts, passwords); err != nil {
		tui.Fatal("运行 TUI 失败", err)
	}
}
