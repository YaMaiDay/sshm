package resource

import "strings"

type ContainerPortMapping struct {
	Name   string
	Target string
}

func AssociatePortContainers(ports []PortDetail, containers []ContainerDetail) {
	portMap := ContainerPublishedPortMap(containers)
	for i := range ports {
		key := strings.ToLower(ports[i].Protocol) + "/" + ports[i].Port
		if mappings := portMap[key]; len(mappings) > 0 {
			names := make([]string, 0, len(mappings))
			targets := make([]string, 0, len(mappings))
			for _, mapping := range mappings {
				if !containsString(names, mapping.Name) {
					names = append(names, mapping.Name)
				}
				if mapping.Target != "" && !containsString(targets, mapping.Target) {
					targets = append(targets, mapping.Target)
				}
			}
			ports[i].Container = strings.Join(names, "、")
			ports[i].ContainerPort = strings.Join(targets, "、")
		}
	}
}

func ContainerPublishedPortMap(containers []ContainerDetail) map[string][]ContainerPortMapping {
	out := map[string][]ContainerPortMapping{}
	for _, container := range containers {
		name := strings.TrimSpace(container.Name)
		if name == "" {
			continue
		}
		for _, part := range strings.Split(container.Ports, ",") {
			hostPort, targetPort, proto, ok := ParseDockerPublishedPort(part)
			if !ok {
				continue
			}
			key := proto + "/" + hostPort
			exists := false
			for _, existing := range out[key] {
				if existing.Name == name && existing.Target == targetPort {
					exists = true
					break
				}
			}
			if !exists {
				out[key] = append(out[key], ContainerPortMapping{Name: name, Target: targetPort})
			}
		}
	}
	return out
}

func ParseDockerPublishedPort(value string) (string, string, string, bool) {
	value = strings.TrimSpace(value)
	left, right, ok := strings.Cut(value, "->")
	if !ok {
		return "", "", "", false
	}
	hostPort := PortFromAddress(left)
	if hostPort == "" {
		return "", "", "", false
	}
	proto := "tcp"
	targetPort := strings.TrimSpace(right)
	if idx := strings.LastIndex(right, "/"); idx >= 0 && idx < len(right)-1 {
		proto = strings.ToLower(strings.TrimSpace(right[idx+1:]))
		targetPort = strings.TrimSpace(right[:idx])
	}
	if proto != "tcp" && proto != "udp" {
		proto = "tcp"
	}
	return hostPort, targetPort, proto, true
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
