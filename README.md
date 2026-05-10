# sshm

全中文终端 SSH 服务器管理器。把服务器管理、实时监控、SSH 登录、文件上传下载放到一个 TUI 界面里完成。

`sshm` 面向习惯使用终端的个人用户：不需要服务器安装 agent，不接管远程 shell，不破坏原生 SSH 体验。登录时直接调用系统 `ssh`，所以远程 Tab 补全、vim、tmux、Ctrl+C 都保持原样。

## 功能

- 全中文 TUI 服务器监控面板
- 服务器分类管理
- 添加、编辑、删除服务器
- CPU、内存、磁盘、负载、运行时间监控
- Docker 运行容器数量和异常服务提示
- 搜索、筛选、排序、手动刷新
- 调用系统 `ssh` 登录服务器
- 双栏文件选择器
- 文件和目录上传
- 文件和目录下载
- 支持密码、密钥、跳板机
- 首次启动可从 OpenSSH 配置迁移
- 修改配置前自动备份

## 安装

macOS / Linux 一键安装：

```sh
curl -fsSL https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.sh | sh
```

指定安装目录：

```sh
curl -fsSL https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.sh | SSHM_INSTALL_DIR="$HOME/.local/bin" sh
```

使用 Go 安装：

```sh
go install github.com/YaMaiDay/sshm/cmd/sshm@latest
```

也可以从 Releases 下载预编译二进制：

```text
https://github.com/YaMaiDay/sshm/releases
```

## 依赖

`sshm` 本身是单个 Go 二进制，但部分能力依赖系统命令：

- `ssh`：登录和监控采集
- `scp`：上传和下载
- `sshpass`：密码登录时建议安装

macOS：

```sh
brew install hudochenkov/sshpass/sshpass
```

Linux：

```sh
sudo apt install openssh-client sshpass
```

## 快速开始

启动：

```sh
sshm
```

第一次打开后，按 `a` 添加服务器。

添加服务器时：

- 左侧填写服务器信息
- 分类只能选择，不能直接输入
- 右侧可以添加或删除分类
- `Tab` 切换左右区域
- `Enter` 保存服务器

常用操作：

```text
↑↓←→       移动
Enter      登录服务器
Space      查看详情
a          添加服务器
e          编辑服务器
x          删除服务器
u          上传文件/目录
d          下载文件/目录
r          手动刷新
/          搜索
t          切换分类
o          只看在线
p          只看异常
s          切换排序
q / Esc    返回或退出
```

## 配置文件

服务器数据保存在：

```text
~/.config/sshm/servers.toml
```

示例：

```toml
categories = ["production", "staging"]

[[servers]]
category = "production"
name = "demo-web"
host = "203.0.113.10"
user = "deploy"
port = 22
key_path = "~/.ssh/id_ed25519"
proxy_jump = ""
password = ""

[[servers]]
category = "staging"
name = "demo-db"
host = "198.51.100.20"
user = "admin"
port = 22
key_path = ""
proxy_jump = ""
password = "example-password"
```

认证方式自动判断：

- `key_path` 不为空时使用密钥
- `password` 不为空时优先使用密码认证
- 两者都为空时交给系统 OpenSSH、ssh-agent 或默认配置处理

应用配置保存在：

```text
~/.config/sshm/config.toml
```

示例：

```toml
refresh_interval = "5s"
connect_timeout = "2s"
command_timeout = "6s"

local_dirs = [".", "~/Downloads", "~/Desktop", "~/Documents", "~"]
remote_dirs = ["$HOME", "/home", "/opt", "/var/www", "/data", "/tmp"]
```

## 数据迁移

如果 `~/.config/sshm/servers.toml` 不存在，首次启动会尝试从这些旧配置迁移：

```text
~/.ssh/config
~/.ssh/config.d/*
~/.ssh/passwords.txt
~/.ssh/passwords/<host>
```

迁移后，添加、编辑、删除、登录、监控、上传和下载都以 `servers.toml` 为准。

每次修改服务器配置前会自动备份：

```text
~/.config/sshm/servers.toml.bak.YYYYMMDD-HHMMSS
```

## 监控方式

`sshm` 通过 SSH 执行远程只读命令采集 Linux 服务器状态，不安装 agent，不修改服务器配置。

采集内容包括：

- `/proc/stat`
- `/proc/loadavg`
- `free`
- `df`
- `uptime`
- `systemctl --failed`
- `docker ps`
- `ss` 或 `netstat`

默认刷新逻辑：

- 每 5 秒刷新一轮
- 单次 SSH 连接最多等待 2 秒
- 单台完整采集最多等待 6 秒
- 在线服务器连续失败 2 次后才显示离线

## 文件传输

上传：

```text
选择服务器 -> 按 u -> 左侧选择本地文件/目录 -> 右侧选择远程目录 -> Space 开始
```

下载：

```text
选择服务器 -> 按 d -> 左侧选择远程文件/目录 -> 右侧选择本地目录 -> Space 开始
```

底层调用系统 `scp`：

```sh
scp file server:/remote/
scp -r dir server:/remote/
scp server:/remote/file local/
scp -r server:/remote/dir local/
```

## 平台支持

当前推荐：

- macOS：Terminal、iTerm2、Ghostty
- Linux：GNOME Terminal、Konsole、Alacritty、Ghostty

实验性支持：

- Windows：Windows Terminal + OpenSSH

Windows 目前可编译运行，但本地路径选择和 `sshpass` 体验没有 macOS/Linux 完整。

## 与 OmnySSH 的差异

`sshm` 的体验参考了 OmnySSH 的终端管理思路，但实现目标不同：

- `sshm` 使用 Go 编写
- `sshm` 调用系统 `ssh`，不内置远程终端
- `sshm` 以中文交互为主
- `sshm` 不要求服务器安装任何东西
- `sshm` 当前使用 `scp` 做传输，不内置 SFTP 协议栈

这样做的取舍是：功能边界更简单，远程 shell 体验更稳定，但跨平台和文件管理能力依赖系统 OpenSSH。

## 开发

运行测试：

```sh
go test ./...
```

本地启动：

```sh
go run ./cmd/sshm
```

构建：

```sh
go build -o sshm ./cmd/sshm
```

常用调试命令：

```sh
go run ./cmd/sshm --list
go run ./cmd/sshm --probe demo-web
go run ./cmd/sshm --remote-dirs demo-web
go run ./cmd/sshm --config-path
```

## 安全边界

`sshm` 默认只做本地配置管理和远程只读监控。

不会做的事：

- 不安装服务器 agent
- 不修改远程 `sshd_config`
- 不自动关闭密码登录
- 不上传密钥
- 不默认扫描 `/root`
- 不内置远程 shell

密码保存在本机 `servers.toml` 中，文件权限设置为 `600`。这只是个人工具的便利设计，不等同于加密保险箱。

## 许可证

MIT
