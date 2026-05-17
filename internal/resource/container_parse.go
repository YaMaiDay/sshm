package resource

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

func ParseContainerDetails(output string) ([]ContainerDetail, string) {
	if strings.Contains(output, "__SSHM_DOCKER_UNAVAILABLE__") {
		return nil, "未安装Docker"
	}
	if strings.Contains(output, "__SSHM_DOCKER_PERMISSION__") {
		return nil, "需要Docker权限（可配置sudo -n docker）"
	}
	lines := strings.Split(output, "\n")
	out := make([]ContainerDetail, 0, len(lines))
	stats := map[string]ContainerDetail{}
	limits := map[string]ContainerDetail{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "__SSHM_CONTAINER_STATS__\t") {
			parts := strings.Split(line, "\t")
			if len(parts) >= 5 {
				name := strings.TrimSpace(parts[1])
				stats[name] = ContainerDetail{
					Name:    name,
					CPU:     strings.TrimSpace(parts[2]),
					Memory:  normalizeDockerMemory(strings.TrimSpace(parts[3])),
					MemPerc: strings.TrimSpace(parts[4]),
				}
			}
			continue
		}
		if strings.HasPrefix(line, "__SSHM_CONTAINER_LIMIT__\t") {
			parts := strings.Split(line, "\t")
			if len(parts) >= 5 {
				name := strings.TrimPrefix(strings.TrimSpace(parts[1]), "/")
				limits[name] = ContainerDetail{
					Name:          name,
					CPULimitKnown: true,
					NanoCpus:      parseContainerLimitInt(parts[2]),
					CPUQuota:      parseContainerLimitInt(parts[3]),
					CPUPeriod:     parseContainerLimitInt(parts[4]),
				}
				if len(parts) >= 6 {
					limit := limits[name]
					limit.CpusetCpus = strings.TrimSpace(parts[5])
					limits[name] = limit
				}
			}
			continue
		}
		line = strings.TrimPrefix(line, "__SSHM_CONTAINER__\t")
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		item := ContainerDetail{
			Name:   strings.TrimSpace(parts[0]),
			Image:  strings.TrimSpace(parts[1]),
			Status: strings.TrimSpace(parts[2]),
		}
		if len(parts) >= 4 {
			item.Ports = strings.TrimSpace(parts[3])
		}
		if stat, ok := stats[item.Name]; ok {
			item.CPU = stat.CPU
			item.Memory = stat.Memory
			item.MemPerc = stat.MemPerc
		}
		if limit, ok := limits[item.Name]; ok {
			item.CPULimitKnown = limit.CPULimitKnown
			item.NanoCpus = limit.NanoCpus
			item.CPUQuota = limit.CPUQuota
			item.CPUPeriod = limit.CPUPeriod
			item.CpusetCpus = limit.CpusetCpus
		}
		if item.Name != "" {
			out = append(out, item)
		}
	}
	for i := range out {
		if stat, ok := stats[out[i].Name]; ok {
			out[i].CPU = stat.CPU
			out[i].Memory = stat.Memory
			out[i].MemPerc = stat.MemPerc
		}
		if limit, ok := limits[out[i].Name]; ok {
			out[i].CPULimitKnown = limit.CPULimitKnown
			out[i].NanoCpus = limit.NanoCpus
			out[i].CPUQuota = limit.CPUQuota
			out[i].CPUPeriod = limit.CPUPeriod
			out[i].CpusetCpus = limit.CpusetCpus
		}
	}
	return out, ""
}

func parseContainerLimitInt(value string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func ParseContainerExtraDetail(output string) (ContainerExtraDetail, string) {
	if strings.Contains(output, "__SSHM_DOCKER_UNAVAILABLE__") {
		return ContainerExtraDetail{}, "未安装Docker"
	}
	if strings.Contains(output, "__SSHM_DOCKER_PERMISSION__") {
		return ContainerExtraDetail{}, "需要Docker权限（可配置sudo -n docker）"
	}
	detail := ContainerExtraDetail{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "__SSHM_CONTAINER_INSPECT__\t"):
			raw := strings.TrimSpace(strings.TrimPrefix(line, "__SSHM_CONTAINER_INSPECT__\t"))
			if err := applyContainerInspectJSON(raw, &detail); err != nil {
				return detail, err.Error()
			}
		case strings.HasPrefix(line, "__SSHM_CONTAINER_SIZE__\t"):
			size := strings.TrimSpace(strings.TrimPrefix(line, "__SSHM_CONTAINER_SIZE__\t"))
			detail.Size, detail.VirtualSize = splitDockerSize(size)
		case strings.HasPrefix(line, "__SSHM_CONTAINER_BLOCKIO__\t"):
			detail.BlockIO = normalizeDockerMemory(strings.TrimSpace(strings.TrimPrefix(line, "__SSHM_CONTAINER_BLOCKIO__\t")))
		}
	}
	return detail, ""
}

func applyContainerInspectJSON(raw string, detail *ContainerExtraDetail) error {
	type inspectItem struct {
		ID         string   `json:"Id"`
		Created    string   `json:"Created"`
		Path       string   `json:"Path"`
		Args       []string `json:"Args"`
		Driver     string   `json:"Driver"`
		Platform   string   `json:"Platform"`
		SizeRw     int64    `json:"SizeRw"`
		SizeRootFS int64    `json:"SizeRootFs"`
		State      struct {
			Status     string `json:"Status"`
			StartedAt  string `json:"StartedAt"`
			FinishedAt string `json:"FinishedAt"`
			ExitCode   int    `json:"ExitCode"`
			Health     *struct {
				Status string `json:"Status"`
			} `json:"Health"`
		} `json:"State"`
		HostConfig struct {
			RestartPolicy struct {
				Name string `json:"Name"`
			} `json:"RestartPolicy"`
			NanoCpus   int64  `json:"NanoCpus"`
			CPUQuota   int64  `json:"CpuQuota"`
			CPUPeriod  int64  `json:"CpuPeriod"`
			CpusetCpus string `json:"CpusetCpus"`
		} `json:"HostConfig"`
		Mounts []struct {
			Type        string `json:"Type"`
			Source      string `json:"Source"`
			Destination string `json:"Destination"`
			RW          bool   `json:"RW"`
		} `json:"Mounts"`
		NetworkSettings struct {
			Networks map[string]struct {
				IPAddress  string   `json:"IPAddress"`
				Gateway    string   `json:"Gateway"`
				MacAddress string   `json:"MacAddress"`
				NetworkID  string   `json:"NetworkID"`
				EndpointID string   `json:"EndpointID"`
				Aliases    []string `json:"Aliases"`
			} `json:"Networks"`
		} `json:"NetworkSettings"`
	}
	var items []inspectItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		var single inspectItem
		if singleErr := json.Unmarshal([]byte(raw), &single); singleErr != nil {
			return err
		}
		items = []inspectItem{single}
	}
	if len(items) == 0 {
		return nil
	}
	item := items[0]
	detail.ID = item.ID
	detail.Created = item.Created
	detail.Path = item.Path
	detail.Args = item.Args
	detail.Driver = item.Driver
	detail.Platform = item.Platform
	detail.StateStatus = item.State.Status
	detail.StartedAt = item.State.StartedAt
	detail.FinishedAt = item.State.FinishedAt
	detail.ExitCode = item.State.ExitCode
	if item.State.Health != nil {
		detail.HealthStatus = item.State.Health.Status
	}
	detail.RestartPolicy = item.HostConfig.RestartPolicy.Name
	detail.NanoCpus = item.HostConfig.NanoCpus
	detail.CPUQuota = item.HostConfig.CPUQuota
	detail.CPUPeriod = item.HostConfig.CPUPeriod
	detail.CpusetCpus = item.HostConfig.CpusetCpus
	if item.SizeRw > 0 {
		detail.SizeRW = uint64(item.SizeRw)
	}
	if item.SizeRootFS > 0 {
		detail.SizeRootFS = uint64(item.SizeRootFS)
	}
	for _, mount := range item.Mounts {
		detail.Mounts = append(detail.Mounts, ContainerMountDetail{
			Type:        mount.Type,
			Source:      mount.Source,
			Destination: mount.Destination,
			RW:          mount.RW,
		})
	}
	for name, network := range item.NetworkSettings.Networks {
		detail.Networks = append(detail.Networks, ContainerNetworkDetail{
			Name:       name,
			IPAddress:  network.IPAddress,
			Gateway:    network.Gateway,
			MacAddress: network.MacAddress,
			NetworkID:  network.NetworkID,
			EndpointID: network.EndpointID,
			Aliases:    network.Aliases,
		})
	}
	sort.Slice(detail.Networks, func(i, j int) bool {
		return detail.Networks[i].Name < detail.Networks[j].Name
	})
	return nil
}

func splitDockerSize(value string) (string, string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ""
	}
	if left, right, ok := strings.Cut(value, "(virtual "); ok {
		return strings.TrimSpace(left), strings.TrimSuffix(strings.TrimSpace(right), ")")
	}
	return value, ""
}

func normalizeDockerMemory(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "GiB", "G")
	value = strings.ReplaceAll(value, "MiB", "M")
	value = strings.ReplaceAll(value, "KiB", "K")
	value = strings.ReplaceAll(value, "TiB", "T")
	value = strings.ReplaceAll(value, " / ", "/")
	value = strings.ReplaceAll(value, "B /", "B/")
	value = strings.ReplaceAll(value, "/ ", "/")
	return value
}
