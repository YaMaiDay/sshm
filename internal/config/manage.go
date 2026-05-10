package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/YaMaiDay/sshm/internal/host"
)

type HostInput struct {
	Category     string
	Name         string
	HostName     string
	User         string
	Port         string
	IdentityFile string
	ProxyJump    string
	Password     string
}

func AddHost(home string, input HostInput) error {
	if err := validateHostInput(input); err != nil {
		return err
	}
	hosts, err := managedHosts(home)
	if err != nil {
		return err
	}
	categories, _, err := LoadCategories(home)
	if err != nil {
		return err
	}
	if !categoryExists(categories, strings.TrimSpace(input.Category)) {
		return fmt.Errorf("分类不存在：%s", input.Category)
	}
	for _, h := range hosts {
		if h.Name == strings.TrimSpace(input.Name) {
			return fmt.Errorf("服务器名称已存在：%s", input.Name)
		}
	}
	hosts = append(hosts, hostFromInput(home, input))
	return SaveServerHosts(home, hosts)
}

func InputFromHost(h host.Host, password string) HostInput {
	if h.Password != "" {
		password = h.Password
	}
	return HostInput{
		Category:     h.Category,
		Name:         h.Name,
		HostName:     h.HostName,
		User:         h.User,
		Port:         h.Port,
		IdentityFile: h.IdentityFile,
		ProxyJump:    h.ProxyJump,
		Password:     password,
	}
}

func EditHost(home string, original host.Host, input HostInput) error {
	if err := validateHostInput(input); err != nil {
		return err
	}
	hosts, err := managedHosts(home)
	if err != nil {
		return err
	}
	categories, _, err := LoadCategories(home)
	if err != nil {
		return err
	}
	if !categoryExists(categories, strings.TrimSpace(input.Category)) {
		return fmt.Errorf("分类不存在：%s", input.Category)
	}
	found := false
	for i, h := range hosts {
		if h.Name == original.Name {
			hosts[i] = hostFromInput(home, input)
			found = true
			continue
		}
		if h.Name == strings.TrimSpace(input.Name) {
			return fmt.Errorf("服务器名称已存在：%s", input.Name)
		}
	}
	if !found {
		return fmt.Errorf("没有找到服务器：%s", original.Name)
	}
	return SaveServerHosts(home, hosts)
}

func categoryExists(categories []string, name string) bool {
	for _, category := range categories {
		if category == name {
			return true
		}
	}
	return false
}

func DeleteHost(home string, h host.Host, removePassword bool) error {
	hosts, err := managedHosts(home)
	if err != nil {
		return err
	}
	next := make([]host.Host, 0, len(hosts))
	found := false
	for _, current := range hosts {
		if current.Name == h.Name {
			found = true
			continue
		}
		next = append(next, current)
	}
	if !found {
		return fmt.Errorf("没有找到服务器：%s", h.Name)
	}
	return SaveServerHosts(home, next)
}

func managedHosts(home string) ([]host.Host, error) {
	if hosts, ok, err := LoadServerHosts(home); ok || err != nil {
		return hosts, err
	}
	hosts, err := LoadHosts(home)
	if err != nil {
		return nil, err
	}
	if err := MigrateServersFile(home, hosts, LoadPasswords(home)); err != nil {
		return nil, err
	}
	hosts, _, err = LoadServerHosts(home)
	return hosts, err
}

func hostFromInput(home string, input HostInput) host.Host {
	password := strings.TrimSpace(input.Password)
	return host.Host{
		Name:         strings.TrimSpace(input.Name),
		HostName:     strings.TrimSpace(input.HostName),
		User:         strings.TrimSpace(input.User),
		Port:         strings.TrimSpace(input.Port),
		IdentityFile: strings.TrimSpace(input.IdentityFile),
		ProxyJump:    strings.TrimSpace(input.ProxyJump),
		Password:     password,
		Category:     strings.TrimSpace(input.Category),
		File:         ServersPath(home),
		HasPassword:  password != "",
	}
}

func validateHostInput(input HostInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return fmt.Errorf("服务器名称不能为空")
	}
	if strings.ContainsAny(input.Name, " \t*?") {
		return fmt.Errorf("服务器名称不能包含空格、* 或 ?")
	}
	if strings.TrimSpace(input.HostName) == "" {
		return fmt.Errorf("服务器地址不能为空")
	}
	if strings.TrimSpace(input.User) == "" {
		return fmt.Errorf("用户名不能为空")
	}
	if strings.TrimSpace(input.Port) == "" {
		return fmt.Errorf("端口不能为空")
	}
	if strings.TrimSpace(input.Category) == "" {
		return fmt.Errorf("分类不能为空")
	}
	return nil
}

func backupIfExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	backup := fmt.Sprintf("%s.bak.%s", path, time.Now().Format("20060102-150405"))
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(backup, data, 0600)
}
