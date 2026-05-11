<p align="center">
  <img src="assets/preview.svg" alt="sshm preview" width="920">
</p>

<h1 align="center">sshm</h1>

<p align="center">
  <strong>全中文终端 SSH 服务器管理器</strong>
  <br>
  把服务器列表、资源监控、SSH 登录、文件传输和日常命令都放进一个 TUI。
</p>

<p align="center">
  <a href="https://github.com/YaMaiDay/sshm/releases"><img alt="Release" src="https://img.shields.io/github/v/release/YaMaiDay/sshm?style=for-the-badge"></a>
  <a href="https://github.com/YaMaiDay/sshm"><img alt="Go" src="https://img.shields.io/badge/Go-1.24-00ADD8?style=for-the-badge&logo=go&logoColor=white"></a>
  <a href="#-平台支持"><img alt="Platform" src="https://img.shields.io/badge/macOS%20%7C%20Linux%20%7C%20Windows-支持-2ea44f?style=for-the-badge"></a>
  <a href="#-执照"><img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue?style=for-the-badge"></a>
</p>

<p align="center">
  <a href="#-安装">安装</a> ·
  <a href="#-快速开始">快速开始</a> ·
  <a href="#-核心功能">功能</a> ·
  <a href="https://github.com/YaMaiDay/sshm/releases">下载</a> ·
  <a href="#-安全边界">安全边界</a>
</p>

---

## 🎯 亮点

一个面板，管服务器、看监控、进 SSH、传文件、跑命令。

- **监控**：CPU、内存、磁盘、负载、容器、端口。
- **登录**：调用系统 `ssh`，保留原生终端体验。
- **管理**：分类、收藏、备注、到期时间、复制服务器。
- **运维**：命令模板、批量命令、命令历史、上传下载。
- **安全**：成功/失败登录摘要、SSH 风险提示。

## ⚡ 安装

安装脚本会下载 GitHub 最新 Release；重复执行同一条命令即可更新。

macOS / Linux：

```sh
curl -fsSL https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.sh | sh
```

Windows PowerShell：

```powershell
irm https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.ps1 | iex
```

运行：

```sh
sshm
sshm --version
```

<details>
<summary>其他安装方式</summary>

macOS 上如果存在 Homebrew 目录，默认会安装到 `/opt/homebrew/bin/sshm`；其他 macOS / Linux 环境默认安装到 `/usr/local/bin/sshm`。可以用 `SSHM_INSTALL_DIR` 覆盖。

如果之前用过本地开发版 alias，当前终端可能还会指向旧路径。可以重新打开终端，或执行：

```sh
unalias sshm 2>/dev/null || true
hash -r
```

指定安装目录：

```sh
curl -fsSL https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.sh | SSHM_INSTALL_DIR="$HOME/.local/bin" sh
```

安装指定版本：

```sh
curl -fsSL https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.sh | SSHM_VERSION=vX.Y.Z sh
```

Windows 指定安装目录：

```powershell
$env:SSHM_INSTALL_DIR="$HOME\bin"; irm https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.ps1 | iex
```

Windows 安装指定版本：

```powershell
$env:SSHM_VERSION="vX.Y.Z"; irm https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.ps1 | iex
```

使用 Go 安装：

```sh
go install github.com/YaMaiDay/sshm/cmd/sshm@latest
```

手动下载：

```text
https://github.com/YaMaiDay/sshm/releases
```

Releases 页面会提供各平台二进制包和 `checksums.txt` 校验文件。

</details>

## ⌨️ 快速开始

第一次打开后按 `a` 添加服务器，`Enter` 保存。添加完成后主面板会自动采集监控数据。

```text
更多    ?
导航    ↑↓←→ / hjkl
登录    Enter        详情    Space
命令    m            批量    b
历史    i            总览    w
收藏    f 收藏       v 只看收藏
管理    a 添加       c 复制       e 编辑       x 删除
传输    u 上传       d 下载
视图    / 搜索       z 视图       Tab 分类     o 在线       p 异常       s 排序
刷新    r            返回    q / Esc
```

主面板底部会优先显示 `更多 ?` 和常用快捷键，按 `?` 可以查看完整快捷键。

## 🚀 核心功能

|  | 功能 | 状态 |
| --- | --- | --- |
| 🖥️ | 全中文 TUI，卡片/分类视图，窄屏自适应 | ✅ |
| 📊 | CPU、内存、磁盘、负载、Swap、inode | ✅ |
| 🐳 | Docker、端口、健康检查、异常服务 | ✅ |
| 🔐 | 系统 `ssh` 登录，保留原生终端体验 | ✅ |
| 🛡️ | 成功/失败登录摘要，SSH 风险提示 | ✅ |
| 🧰 | 命令模板、批量命令、命令历史 | ✅ |
| 📁 | 文件和目录上传/下载 | ✅ |
| 🗂️ | 分类、收藏、备注、到期时间、复制服务器 | ✅ |
| 🔄 | 从 OpenSSH 配置迁移 | ✅ |
| 🧪 | Windows Terminal + OpenSSH | 实验性 |

## 📦 依赖

`sshm` 本身是单个 Go 二进制，但部分能力依赖系统命令：

| 依赖 | 用途 |
| --- | --- |
| `ssh` | 登录服务器、采集监控信息 |
| `scp` | 上传和下载文件 |
| `sshpass` | 密码登录，非必需但建议安装 |

macOS：

```sh
brew install hudochenkov/sshpass/sshpass
```

Debian / Ubuntu：

```sh
sudo apt install openssh-client sshpass
```

## 🗂️ 文件位置

一键安装只安装程序本体。配置、模板、历史都放在用户目录，不会写进项目目录。

macOS / Linux：

```sh
~/.config/sshm/servers.toml    # 服务器
~/.config/sshm/commands.toml   # 命令模板
~/.config/sshm/history.toml    # 命令历史
~/.config/sshm/config.toml     # 应用配置
~/.config/sshm/state.toml      # 本机状态
```

Windows：

```text
%USERPROFILE%\.config\sshm\servers.toml
%USERPROFILE%\.config\sshm\commands.toml
%USERPROFILE%\.config\sshm\history.toml
%APPDATA%\sshm\config.toml
```

<details>
<summary>查看完整文件位置</summary>

| 类型 | macOS / Linux | Windows |
| --- | --- | --- |
| 程序本体 | `/opt/homebrew/bin/sshm` 或 `/usr/local/bin/sshm` | `%LOCALAPPDATA%\Programs\sshm\sshm.exe` |
| 服务器配置 | `~/.config/sshm/servers.toml` | `%USERPROFILE%\.config\sshm\servers.toml` |
| 命令模板 | `~/.config/sshm/commands.toml` | `%USERPROFILE%\.config\sshm\commands.toml` |
| 命令历史 | `~/.config/sshm/history.toml` | `%USERPROFILE%\.config\sshm\history.toml` |
| 应用配置 | `~/.config/sshm/config.toml` | `%APPDATA%\sshm\config.toml` |
| 本机状态 | `~/.config/sshm/state.toml` | `%USERPROFILE%\.config\sshm\state.toml` |

打开配置目录：

```sh
open ~/.config/sshm
```

</details>

## ⚙️ 配置

大部分配置都可以在 TUI 里完成，手动改文件也可以。

| 文件 | 用途 |
| --- | --- |
| `servers.toml` | 服务器、分类、密钥、密码、备注、到期时间、健康端口 |
| `commands.toml` | 全局模板、单服务器模板 |
| `config.toml` | 刷新间隔、连接超时、本地/远程常用目录 |

认证逻辑：

- 配了 `key_path`：只用当前服务器配置的密钥。
- 配了 `password`：允许密码和 PAM。
- 密钥和密码都有：先密钥，后密码。
- 两者都没有：交给系统 OpenSSH / ssh-agent。

<details>
<summary>查看配置示例</summary>

`servers.toml`

```toml
categories = ["production", "staging"]

[[servers]]
category = "production"
name = "demo-web"
host = "203.0.113.10"
user = "deploy"
port = 22
key_path = "~/.ssh/id_ed25519"
password = ""
note = "线上 Web 入口"
expire_at = "2026-08-31"
favorite = true
health_ports = [80, 443, 8080]
```

`commands.toml`

```toml
[[global]]
name = "查看容器"
command = """
docker ps -a
"""

[[server]]
server = "production/demo-web"
name = "更新项目"
command = """
cd /home/app
git pull
docker compose up -d
"""
```

`config.toml`

```toml
refresh_interval = "5s"
connect_timeout = "2s"
command_timeout = "6s"

local_dirs = ["~/Downloads", "~/Desktop", "~"]
remote_dirs = ["$HOME", "/home", "/opt", "/var/www", "/data", "/tmp"]
```

</details>

## 🧰 命令模板

主面板或详情页按 `m` 打开命令模板。

| 操作 | 说明 |
| --- | --- |
| 单台模板 | 当前服务器执行 |
| 全局模板 | 所有服务器可用 |
| 临时命令 | 不保存，执行一次 |
| 批量命令 | 按 `b` 选择多台服务器串行执行 |
| 命令历史 | 按 `i` 查看输出、退出码、重新执行 |

模板编辑里 `Ctrl+J` 换行，执行前会显示完整命令并要求确认。

<details>
<summary>查看命令历史规则</summary>

- 单台命令、临时命令、批量命令执行完成后都会记录。
- 历史里可以查看执行时间、目标服务器、命令内容、退出码和输出。
- `r` 可以按历史里保存的命令重新执行；如果服务器已删除，会提示不能执行。
- `x` 删除当前历史记录，删除前会二次确认。
- 默认最多保留最近 100 条历史，每台服务器输出只保存最后 200 行，避免历史文件过大。

</details>

## 🔄 数据迁移

首次启动时，如果没有 `servers.toml`，会尝试从 OpenSSH 配置和旧密码文件迁移。

<details>
<summary>查看迁移来源</summary>

```text
~/.ssh/config
~/.ssh/config.d/*
~/.ssh/passwords.txt
~/.ssh/passwords/<host>
```

迁移后，添加、编辑、删除、登录、监控、上传和下载都以 `servers.toml` 为准。保存配置时会直接覆盖 `servers.toml`，不会自动生成 `.bak` 备份文件。

</details>

## 🖥️ 登录体验

登录时直接运行系统 `ssh` 或 `sshpass ssh`，并临时让出终端控制权。

进入服务器后就是原生 SSH：

- `vim`、`top`、`htop`、`tmux` 正常运行。
- `Ctrl+C`、方向键、删除键、Tab 补全都按远程 shell 规则工作。
- 退出远程服务器后回到 `sshm` 面板。

如果本机 OpenSSH 支持 `WarnWeakCrypto`，`sshm` 会自动隐藏下面这类 post-quantum 提示：

```text
WARNING: connection is not using a post-quantum key exchange algorithm
```

连接失败、认证失败、网络错误等正常错误仍会显示。

## 📡 监控方式

通过 SSH 执行远程只读命令采集状态，不安装 agent，不修改服务器配置。

| 默认策略 | 值 |
| --- | --- |
| 自动刷新 | 每 5 秒一轮 |
| SSH 连接超时 | 2 秒 |
| 单台采集超时 | 6 秒 |
| 离线判断 | 在线服务器连续失败 2 次后显示离线 |

| 面板 | 说明 |
| --- | --- |
| 卡片视图 | 看资源概览 |
| 分类视图 | 按分类查看服务器，窄屏自动切顶部分类 |
| 详情页 | 基础、资源、服务、容器、登录、风险分组 |
| 异常总览 | 按严重和警告数量优先查看问题服务器 |

<details>
<summary>查看采集和风险检查细节</summary>

- 资源：CPU、内存、Swap、磁盘、inode、负载、运行时间。
- 服务：健康端口、监听端口、异常 systemd 服务。
- 容器：`docker ps -a` 统计运行、停止、故障容器。
- 登录：成功登录用 `last -n 100`，失败登录用 `lastb -n 100`，权限不足时尝试 `sudo -n lastb -n 100`。
- 风险：到期时间、资源使用率、健康端口、故障容器、异常服务、失败登录数量、密码登录、Root 登录。
- 采集命令：`/proc/stat`、`/proc/loadavg`、`free`、`df`、`uname`、`uptime`、`systemctl --failed`、`docker ps -a`、`ss` 或 `netstat`。

</details>

## 📁 文件传输

| 操作 | 流程 |
| --- | --- |
| 上传 | 选择服务器 -> 按 `u` -> 左侧选择本地文件/目录 -> 右侧选择远程目录 -> `Space` 开始 |
| 下载 | 选择服务器 -> 按 `d` -> 左侧选择远程文件/目录 -> 右侧选择本地目录 -> `Space` 开始 |

底层调用系统 `scp`，支持文件和目录。传输过程中按 `q` / `Esc` / `Ctrl+C` 退出 `sshm` 时，会主动中断正在运行的上传或下载子进程。

## 💻 平台支持

macOS / Linux 推荐使用。Windows Terminal + OpenSSH 可用，但本地路径选择和 `sshpass` 体验还不如 macOS/Linux 完整。

## 🧭 设计取舍

| 项目 | 说明 |
| --- | --- |
| 开发语言 | Go |
| 交互语言 | 中文优先 |
| SSH 登录 | 调用系统 `ssh` |
| 远程 shell | 不内置，保留原生终端体验 |
| 远程依赖 | 不安装 agent |
| 文件传输 | 调用系统 `scp` |

## 🛠️ 开发

拉取源码：

```sh
git clone https://github.com/YaMaiDay/sshm.git
cd sshm
```

```sh
go test ./...
go run ./cmd/sshm
go build -o sshm ./cmd/sshm
go run ./cmd/sshm --version
```

常用调试命令：

```sh
go run ./cmd/sshm --list
go run ./cmd/sshm --probe demo-web
go run ./cmd/sshm --remote-dirs demo-web
go run ./cmd/sshm --config-path
```

<details>
<summary>源码结构</summary>

```text
sshm/
├── cmd/sshm/main.go              # CLI 入口
├── internal/actions/             # SSH 登录、上传下载动作
├── internal/config/              # 配置读取、迁移、服务器管理
├── internal/fsselect/            # 本地/远程文件选择
├── internal/host/                # 服务器数据结构
├── internal/monitor/             # SSH 监控采集与指标解析
├── internal/tui/                 # Bubble Tea TUI 界面
├── assets/preview.svg            # README 预览图
├── install.sh                    # macOS / Linux 安装脚本
├── install.ps1                   # Windows 安装脚本
└── README.md
```

</details>

## 🔒 安全边界

`sshm` 默认只做本地配置管理和远程只读监控。

不会做的事：

- 不安装服务器 agent
- 不修改远程 `sshd_config`
- 不自动关闭密码登录
- 不上传密钥
- 不默认扫描 `/root`
- 不内置远程 shell

密码保存在本机 `servers.toml` 中，文件权限设置为 `600`。这只是个人工具的便利设计，不等同于加密保险箱。

## 📄 执照

Apache 2.0 — 请参阅 [LICENSE](LICENSE)。

---

<p align="center">
  由 <a href="https://github.com/YaMaiDay">YaMaiDay</a> 用心制作 ❤️
</p>

<p align="center">
  ⭐ 如果你觉得这个仓库有用，请给它点个星！ ⭐
</p>
