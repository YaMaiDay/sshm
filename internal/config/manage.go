package config

import (
	"fmt"
	"strconv"
	"strings"

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
	Favorite     bool
	HealthPorts  []int
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
		if sameHostNameInCategory(h, input.Category, input.Name) {
			return fmt.Errorf("分类 %s 中服务器名称已存在：%s", strings.TrimSpace(input.Category), strings.TrimSpace(input.Name))
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
		Favorite:     h.Favorite,
		HealthPorts:  normalizeHealthPorts(h.HealthPorts),
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
		if sameHostIdentity(h, original) {
			hosts[i] = hostFromInput(home, input)
			found = true
			continue
		}
		if sameHostNameInCategory(h, input.Category, input.Name) {
			return fmt.Errorf("分类 %s 中服务器名称已存在：%s", strings.TrimSpace(input.Category), strings.TrimSpace(input.Name))
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
		if sameHostIdentity(current, h) {
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
		Favorite:     input.Favorite,
		HealthPorts:  normalizeHealthPorts(input.HealthPorts),
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

func ParseHealthPorts(value string) ([]int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	})
	ports := make([]int, 0, len(parts))
	seen := map[int]bool{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		port, err := strconv.Atoi(part)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("健康端口无效：%s", part)
		}
		if !seen[port] {
			ports = append(ports, port)
			seen[port] = true
		}
	}
	return ports, nil
}

func FormatHealthPorts(ports []int) string {
	ports = normalizeHealthPorts(ports)
	if len(ports) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ports))
	for _, port := range ports {
		parts = append(parts, strconv.Itoa(port))
	}
	return strings.Join(parts, ",")
}

func normalizeHealthPorts(ports []int) []int {
	if len(ports) == 0 {
		return nil
	}
	out := make([]int, 0, len(ports))
	seen := map[int]bool{}
	for _, port := range ports {
		if port < 1 || port > 65535 || seen[port] {
			continue
		}
		out = append(out, port)
		seen[port] = true
	}
	return out
}

func sameHostNameInCategory(h host.Host, category, name string) bool {
	return strings.TrimSpace(h.Category) == strings.TrimSpace(category) &&
		strings.TrimSpace(h.Name) == strings.TrimSpace(name)
}

func sameHostIdentity(a, b host.Host) bool {
	return strings.TrimSpace(a.Category) == strings.TrimSpace(b.Category) &&
		strings.TrimSpace(a.Name) == strings.TrimSpace(b.Name)
}
