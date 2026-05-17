package resource

import (
	"strings"
	"testing"
)

func TestParsePortDetailsSSOutput(t *testing.T) {
	output := strings.Join([]string{
		`tcp LISTEN 0 4096 0.0.0.0:22 0.0.0.0:* users:(("sshd",pid=123,fd=3))`,
		`tcp LISTEN 0 4096 [::]:22 [::]:* users:(("sshd",pid=123,fd=4))`,
		`udp UNCONN 0 0 127.0.0.1:323 0.0.0.0:* users:(("chronyd",pid=456,fd=5))`,
		`udp UNCONN 0 0 [::1]:323 [::]:* users:(("chronyd",pid=456,fd=6))`,
		`tcp LISTEN 0 511 *:80 *:* users:(("nginx",pid=789,fd=6))`,
		`tcp LISTEN 0 4096 [::]:443 [::]:* users:(("caddy",pid=987,fd=4))`,
		`tcp 0 4096 0.0.0.0:8080 0.0.0.0:*`,
		`__SSHM_PORT_CGROUP__	789	nginx.service`,
	}, "\n")
	ports, errText := ParsePortDetails(output)
	if errText != "" {
		t.Fatalf("errText = %q", errText)
	}
	if len(ports) != 5 {
		t.Fatalf("ports = %#v, want 5", ports)
	}
	if ports[0].Port != "22" || ports[0].LocalAddress != "0.0.0.0:22, [::]:22" || ports[0].State != "LISTEN" || ports[0].Process != "sshd" || ports[0].PID != "123" || ports[0].FD != "3, 4" {
		t.Fatalf("first port = %+v, want sshd on 22", ports[0])
	}
	if ports[3].Port != "443" || ports[3].Process != "caddy" || ports[3].PID != "987" {
		t.Fatalf("fourth port = %+v, want caddy on 443", ports[3])
	}
	var nginxPort PortDetail
	for _, port := range ports {
		if port.Port == "80" {
			nginxPort = port
			break
		}
	}
	if nginxPort.Port != "80" || nginxPort.ServiceUnit != "nginx.service" {
		t.Fatalf("nginx port service unit = %+v, want nginx.service", nginxPort)
	}
	if ports[4].Port != "8080" || ports[4].Process != "" || ports[4].PID != "" {
		t.Fatalf("fifth port = %+v, want unnamed 8080", ports[4])
	}
}

func TestParsePortDetailsNetstatOutput(t *testing.T) {
	output := strings.Join([]string{
		`Proto Recv-Q Send-Q Local Address           Foreign Address         State       PID/Program name`,
		`tcp        0      0 0.0.0.0:22              0.0.0.0:*               LISTEN      123/sshd`,
		`tcp6       0      0 :::22                   :::*                    LISTEN      123/sshd`,
		`udp        0      0 127.0.0.1:323           0.0.0.0:*                           456/chronyd`,
		`udp6       0      0 ::1:323                 :::*                                456/chronyd`,
	}, "\n")
	ports, errText := ParsePortDetails(output)
	if errText != "" {
		t.Fatalf("errText = %q", errText)
	}
	if len(ports) != 2 {
		t.Fatalf("ports = %#v, want 2", ports)
	}
	if ports[0].Protocol != "tcp" || ports[0].Port != "22" || ports[0].LocalAddress != "0.0.0.0:22, :::22" || ports[0].State != "LISTEN" || ports[0].Process != "sshd" || ports[0].PID != "123" {
		t.Fatalf("first port = %+v, want sshd on 22", ports[0])
	}
	if ports[1].Protocol != "udp" || ports[1].Port != "323" || ports[1].LocalAddress != "127.0.0.1:323, ::1:323" || ports[1].Process != "chronyd" || ports[1].PID != "456" {
		t.Fatalf("second port = %+v, want chronyd on 323", ports[1])
	}
}
