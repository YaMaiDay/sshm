# 配置与数据

## 服务器数据

`sshm` 使用一个独立文件保存服务器：

```text
~/.config/sshm/servers.toml
```

示例：

```toml
[[servers]]
category = "production"
name = "demo-app"
host = "203.0.113.20"
user = "deploy"
port = 22
key_path = "~/.ssh/production.pem"
proxy_jump = ""
password = ""

[[servers]]
category = "staging"
name = "demo-db"
host = "198.51.100.30"
user = "root"
port = 22
key_path = ""
proxy_jump = ""
password = "your-password"
```

认证方式不需要单独写 `auth_type`：

- `key_path` 不为空时使用密钥
- `password` 不为空时使用密码，并优先限制 SSH 只走密码认证
- 两者都为空时交给系统 OpenSSH、ssh-agent 或默认配置处理

服务器文件权限自动设置为：

```text
chmod 600 ~/.config/sshm/servers.toml
```

## 旧数据迁移

首次启动时，如果 `servers.toml` 不存在，程序会尝试从旧配置迁移：

```text
~/.ssh/config
~/.ssh/config.d/*
~/.ssh/passwords.txt
~/.ssh/passwords/<host>
```

迁移完成后，`sshm` 的添加、编辑、删除、登录、监控、上传和下载都以 `servers.toml` 为准。

## 应用配置

应用配置保存 UI 和行为设置：

macOS/Linux：

```text
~/.config/sshm/config.toml
```

Windows：

```text
%APPDATA%\sshm\config.toml
```

示例：

```toml
refresh_interval = "5s"
connect_timeout = "2s"
command_timeout = "6s"
ascii_mode = false

local_dirs = [".", "~/Downloads", "~/Desktop", "~/Documents", "~"]
remote_dirs = ["$HOME", "/home", "/opt", "/var/www", "/www", "/data", "/tmp"]

[thresholds]
cpu_warn = 70
cpu_crit = 85
mem_warn = 70
mem_crit = 85
disk_warn = 75
disk_crit = 90
```

当前监控刷新逻辑：

- 每 5 秒全量刷新一次所有服务器
- 单次 SSH 连接最多等待 2 秒
- 单台完整采集最多等待 6 秒

## 备份

每次修改 `servers.toml` 前自动备份：

```text
~/.config/sshm/servers.toml.bak.20260511-020000
```
