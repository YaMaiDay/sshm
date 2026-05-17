package tui

import (
	"sort"
	"strings"

	resourceservice "github.com/YaMaiDay/sshm/internal/resource"
)

func deriveDatabaseDetails(services []resourceservice.ServiceDetail, containers []resourceservice.ContainerDetail, ports []resourceservice.PortDetail) ([]resourceservice.DatabaseDetail, string) {
	items := []resourceservice.DatabaseDetail{}
	for _, item := range containers {
		engine := databaseEngineFromContainer(item)
		if engine == "" {
			continue
		}
		db := resourceservice.DatabaseDetail{
			Name:      firstNonEmpty(strings.TrimSpace(item.Name), engine),
			Engine:    engine,
			Source:    "Docker",
			Status:    databaseStatusFromContainer(item),
			RawStatus: item.Status,
			Endpoint:  firstDatabaseEndpointFromPorts(item.Ports, engine),
			Container: item.Name,
			Image:     item.Image,
		}
		items = append(items, db)
	}
	for _, item := range services {
		engine := databaseEngineFromService(item)
		if engine == "" {
			continue
		}
		db := resourceservice.DatabaseDetail{
			Name:        firstNonEmpty(strings.TrimSuffix(strings.TrimSpace(item.Unit), ".service"), engine),
			Engine:      engine,
			Source:      "systemd",
			Status:      databaseStatusFromService(item),
			RawStatus:   serviceRawState(item),
			ServiceUnit: item.Unit,
			Process:     serviceProgramName(item),
			PID:         servicePIDText(item),
		}
		items = mergeDatabaseDetail(items, db)
	}
	for _, item := range ports {
		engine := databaseEngineFromPort(item)
		if engine == "" {
			continue
		}
		db := resourceservice.DatabaseDetail{
			Name:        databaseNameFromPort(item, engine),
			Engine:      engine,
			Source:      databasePortSource(item),
			Status:      databaseStatusFromPort(item),
			RawStatus:   emptyDash(item.State),
			Endpoint:    portListenText(item),
			ServiceUnit: item.ServiceUnit,
			Container:   item.Container,
			Process:     item.Process,
			PID:         item.PID,
			Protocol:    item.Protocol,
			Port:        item.Port,
		}
		items = mergeDatabaseDetail(items, db)
	}
	sort.SliceStable(items, func(i, j int) bool {
		ri, rj := databaseStatusRank(items[i]), databaseStatusRank(items[j])
		if ri != rj {
			return ri < rj
		}
		if items[i].Engine != items[j].Engine {
			return items[i].Engine < items[j].Engine
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	return items, ""
}

func mergeDatabaseDetail(items []resourceservice.DatabaseDetail, next resourceservice.DatabaseDetail) []resourceservice.DatabaseDetail {
	for i := range items {
		if !sameDatabaseResource(items[i], next) {
			continue
		}
		items[i] = combineDatabaseDetail(items[i], next)
		return items
	}
	return append(items, next)
}

func sameDatabaseResource(a resourceservice.DatabaseDetail, b resourceservice.DatabaseDetail) bool {
	if a.Engine != b.Engine {
		return false
	}
	if nonEmptyEqual(a.Container, b.Container) || nonEmptyEqual(a.ServiceUnit, b.ServiceUnit) {
		return true
	}
	if nonEmptyEqual(a.Process, b.Process) && nonEmptyEqual(a.PID, b.PID) {
		return true
	}
	if a.Port != "" && b.Port != "" && a.Port == b.Port && strings.EqualFold(a.Protocol, b.Protocol) {
		return true
	}
	if resourceservice.DatabaseDefaultPort(a.Engine) != "" && (a.Port == resourceservice.DatabaseDefaultPort(a.Engine) || b.Port == resourceservice.DatabaseDefaultPort(a.Engine)) {
		return a.Source != "Docker" && b.Source != "Docker"
	}
	return false
}

func combineDatabaseDetail(a resourceservice.DatabaseDetail, b resourceservice.DatabaseDetail) resourceservice.DatabaseDetail {
	a.Name = firstNonEmpty(a.Name, b.Name)
	a.Source = combineDatabaseSource(a.Source, b.Source)
	a.Status = databaseWorseStatus(a.Status, b.Status)
	a.RawStatus = firstNonEmpty(a.RawStatus, b.RawStatus)
	a.Endpoint = firstNonEmpty(a.Endpoint, b.Endpoint)
	a.ServiceUnit = firstNonEmpty(a.ServiceUnit, b.ServiceUnit)
	a.Container = firstNonEmpty(a.Container, b.Container)
	a.Image = firstNonEmpty(a.Image, b.Image)
	a.Process = firstNonEmpty(a.Process, b.Process)
	a.PID = firstNonEmpty(a.PID, b.PID)
	a.Protocol = firstNonEmpty(a.Protocol, b.Protocol)
	a.Port = firstNonEmpty(a.Port, b.Port)
	return a
}

func combineDatabaseSource(a string, b string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		return b
	}
	if b == "" || strings.Contains(a, b) {
		return a
	}
	if strings.Contains(b, a) {
		return b
	}
	return a + "+" + b
}

func databaseWorseStatus(a string, b string) string {
	if databaseStatusRank(resourceservice.DatabaseDetail{Status: b}) < databaseStatusRank(resourceservice.DatabaseDetail{Status: a}) {
		return b
	}
	return a
}

func databaseEngineFromContainer(item resourceservice.ContainerDetail) string {
	text := strings.ToLower(strings.Join([]string{item.Name, item.Image}, " "))
	return databaseEngineFromText(text)
}

func databaseEngineFromService(item resourceservice.ServiceDetail) string {
	text := strings.ToLower(strings.Join([]string{item.Unit, item.Description, item.ExecStart, serviceProgramPath(item)}, " "))
	return databaseEngineFromText(text)
}

func databaseEngineFromPort(item resourceservice.PortDetail) string {
	if engine := resourceservice.DatabaseEngineFromDefaultPort(item.Port); engine != "" {
		return engine
	}
	text := strings.ToLower(strings.Join([]string{item.Process, item.ServiceUnit, item.Container}, " "))
	return databaseEngineFromText(text)
}

func databaseEngineFromText(text string) string {
	switch {
	case containsDatabaseToken(text, "mariadb"), containsDatabaseToken(text, "mariadbd"):
		return "MariaDB"
	case containsDatabaseToken(text, "mysql"), containsDatabaseToken(text, "mysqld"):
		return "MySQL"
	case containsDatabaseToken(text, "postgres"), containsDatabaseToken(text, "postgresql"):
		return "PostgreSQL"
	case containsDatabaseToken(text, "redis"), containsDatabaseToken(text, "redis-server"):
		return "Redis"
	case containsDatabaseToken(text, "mongodb"), containsDatabaseToken(text, "mongo"), containsDatabaseToken(text, "mongod"):
		return "MongoDB"
	default:
		return ""
	}
}

func containsDatabaseToken(text string, token string) bool {
	text = strings.ToLower(text)
	token = strings.ToLower(token)
	for _, part := range strings.FieldsFunc(text, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	}) {
		if part == token {
			return true
		}
	}
	return strings.Contains(text, "/"+token+":") || strings.Contains(text, ":"+token+":")
}

func databaseStatusFromContainer(item resourceservice.ContainerDetail) string {
	switch containerDetailKind(item) {
	case "failed":
		return "problem"
	case "stopped":
		return "stopped"
	case "missing":
		return "missing"
	case "running":
		return "running"
	default:
		return "unknown"
	}
}

func databaseStatusFromService(item resourceservice.ServiceDetail) string {
	switch serviceDetailKind(item) {
	case "failed":
		return "problem"
	case "stopped":
		return "stopped"
	case "missing":
		return "missing"
	case "running", "active":
		return "running"
	default:
		return "unknown"
	}
}

func databaseStatusFromPort(item resourceservice.PortDetail) string {
	if item.Missing {
		return "missing"
	}
	state := strings.ToUpper(strings.TrimSpace(item.State))
	if state == "" || state == "LISTEN" || state == "UNCONN" {
		return "running"
	}
	return "unknown"
}

func databaseStatusRank(item resourceservice.DatabaseDetail) int {
	switch item.Status {
	case "problem", "missing":
		return 0
	case "running":
		return 1
	case "stopped":
		return 2
	default:
		return 3
	}
}

func firstDatabaseEndpointFromPorts(ports string, engine string) string {
	defaultPort := resourceservice.DatabaseDefaultPort(engine)
	for _, part := range strings.Split(ports, ",") {
		hostPort, _, _, ok := resourceservice.ParseDockerPublishedPort(part)
		if ok && (defaultPort == "" || hostPort == defaultPort) {
			return strings.TrimSpace(part)
		}
	}
	return strings.TrimSpace(ports)
}

func databaseNameFromPort(item resourceservice.PortDetail, engine string) string {
	return firstNonEmpty(item.Container, item.ServiceUnit, item.Process, engine)
}

func databasePortSource(item resourceservice.PortDetail) string {
	if strings.TrimSpace(item.Container) != "" {
		return "Docker+port"
	}
	if strings.TrimSpace(item.ServiceUnit) != "" {
		return "systemd+port"
	}
	return "port"
}

func serviceProgramName(item resourceservice.ServiceDetail) string {
	path := serviceProgramPath(item)
	if path == "" {
		return ""
	}
	if idx := strings.LastIndex(path, "/"); idx >= 0 && idx < len(path)-1 {
		return path[idx+1:]
	}
	return path
}

func nonEmptyEqual(a string, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	return a != "" && b != "" && strings.EqualFold(a, b)
}

func databaseDefaultHost(item resourceservice.DatabaseDetail) string {
	host, _, ok := databasePublishedEndpoint(item)
	if ok {
		return host
	}
	return "127.0.0.1"
}

func databasePublishedEndpoint(item resourceservice.DatabaseDetail) (string, string, bool) {
	defaultPort := resourceservice.DatabaseDefaultPort(item.Engine)
	for _, part := range strings.Split(item.Endpoint, ",") {
		hostPort, targetPort, _, ok := resourceservice.ParseDockerPublishedPort(part)
		if ok && (defaultPort == "" || targetPort == defaultPort) {
			return "127.0.0.1", hostPort, true
		}
	}
	if strings.TrimSpace(item.Port) != "" {
		return "127.0.0.1", strings.TrimSpace(item.Port), true
	}
	return "", "", false
}

func databaseDefaultPortForDetail(item resourceservice.DatabaseDetail) string {
	_, port, ok := databasePublishedEndpoint(item)
	if ok {
		return port
	}
	endpoint := strings.TrimSpace(item.Endpoint)
	if endpoint != "" && !strings.Contains(endpoint, "->") {
		port := resourceservice.PortFromAddress(endpoint)
		if port != "" {
			return port
		}
	}
	if strings.TrimSpace(item.Port) != "" {
		return strings.TrimSpace(item.Port)
	}
	return resourceservice.DatabaseDefaultPort(item.Engine)
}
