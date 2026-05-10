# 技术设计

## 技术栈

语言：Go

TUI：

- Bubble Tea
- Lip Gloss
- Bubbles

配置：

- pelletier/go-toml

外部命令：

- `ssh`
- `scp`
- 可选 `sshpass`

## 项目结构

```text
sshm/
  cmd/sshm/main.go

  internal/config/
    sshconfig.go
    servers.go
    appconfig.go
    passwords.go

  internal/host/
    host.go
    category.go

  internal/monitor/
    collector.go
    command.go
    metrics.go
    parser.go
    worker.go

  internal/tui/
    model.go
    update.go
    view.go
    table.go
    styles.go
    help.go

  internal/actions/
    ssh.go
    transfer.go
    command.go

  internal/fsselect/
    local.go
    remote.go
```

## 服务器配置

主配置文件：

```text
~/.config/sshm/servers.toml
```

支持字段：

- category
- name
- host
- user
- port
- key_path
- password

首次启动如果 `servers.toml` 不存在，会从已有 OpenSSH 配置和旧密码文件迁移一次。迁移后添加、编辑、删除均只写 `servers.toml`。

服务器数据结构：

```go
type Host struct {
    Category     string
    Name         string
    HostName     string
    User         string
    Port         string
    IdentityFile string
    ProxyJump    string
    Password     string
}
```

## 监控采集

采集方式：通过 SSH 执行远程只读脚本。

刷新策略：

- 默认 5 秒刷新
- 每轮全量采集所有服务器
- 连接超时 2 秒
- 命令超时 6 秒
- 在线服务器连续失败 2 次后才显示离线
- 手动刷新立即触发全量采集

Linux 远程采集命令：

```sh
cat /etc/os-release 2>/dev/null
cat /proc/stat
sleep 0.5
cat /proc/stat
cat /proc/loadavg
free -b
df -P /
uptime -p 2>/dev/null || uptime
nproc 2>/dev/null
systemctl --failed --no-pager --plain 2>/dev/null
docker ps --format '{{.Names}}' 2>/dev/null
ss -tln 2>/dev/null || netstat -tln 2>/dev/null
```

CPU 使用率通过两次 `/proc/stat` 差值计算。

## SSH 调用

不内置 SSH 协议，优先调用系统 `ssh`。

普通主机：

```sh
ssh -o ConnectTimeout=2 -o LogLevel=ERROR user@host remote-script
```

密码主机：

```sh
sshpass -f temp ssh -o PreferredAuthentications=password -o PubkeyAuthentication=no user@host remote-script
```

可选连接复用：

```sh
-o ControlMaster=auto
-o ControlPath=~/.ssh/sshm-%r@%h:%p
-o ControlPersist=60s
```

## TUI 状态模型

```go
type HostState struct {
    Host     Host
    Metrics  Metrics
    Online   bool
    Loading  bool
    LastErr  string
    Updated  time.Time
}
```

TUI 每秒刷新视图，但指标由 worker 异步更新。

## 登录动作

登录时：

1. 暂停 TUI
2. 恢复终端原始模式
3. 调用系统 `ssh Host`
4. SSH 退出后恢复 TUI

避免内置 PTY，避免 Tab 补全冲突。

## 文件传输

不内置 SFTP client，调用系统 `scp`。

Go 中必须使用 `exec.Command` 参数数组，不拼 shell 字符串，保证路径空格和 Windows 路径安全。
