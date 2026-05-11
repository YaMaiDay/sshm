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
  <a href="#-一眼能做什么">一眼能做什么</a> ·
  <a href="#-快速开始">快速开始</a> ·
  <a href="#-核心功能">功能</a> ·
  <a href="https://github.com/YaMaiDay/sshm/releases">下载</a> ·
  <a href="#-安全边界">安全边界</a>
</p>

---

## 🎯 一眼能做什么

一个面板，管服务器、看监控、进 SSH、传文件、跑命令。

```sh
curl -fsSL https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.sh | sh
sshm
```

Windows PowerShell：

```powershell
irm https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.ps1 | iex
sshm
```

核心体验：

- **监控**：CPU、内存、磁盘、负载、容器、端口。
- **登录**：调用系统 `ssh`，保留原生终端体验。
- **管理**：分类、收藏、备注、到期时间、复制服务器。
- **运维**：命令模板、批量命令、命令历史、上传下载。
- **安全**：成功/失败登录摘要、SSH 风险提示。

## ✨ 为什么用 sshm

如果你习惯待在终端里，`sshm` 可以把“找服务器、看状态、登录、传文件、执行命令”这些动作压到一个入口里。

| 以前 | 现在 |
| --- | --- |
| 找 IP、端口、用户名 | 统一服务器面板 |
| 登录后手动 `top` / `df` | 主面板直接看监控 |
| 重复敲部署命令 | 保存模板或批量执行 |
| 手写 `scp` | 双栏选择上传下载 |
| 想看 SSH 是否被扫 | 详情页看登录和风险 |

## ⚡ 安装

安装脚本会下载 GitHub 最新 Release。重复执行同一条命令即可更新到最新版。

### macOS / Linux

```sh
curl -fsSL https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.sh | sh
```

macOS 上如果存在 Homebrew 目录，默认会安装到 `/opt/homebrew/bin/sshm`；其他 macOS / Linux 环境默认安装到 `/usr/local/bin/sshm`。可以用 `SSHM_INSTALL_DIR` 覆盖。

### Windows PowerShell（测试阶段）

```powershell
irm https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.ps1 | iex
```

安装完成后运行：

```sh
sshm
```

查看当前版本：

```sh
sshm --version
```

如果之前用过本地开发版 alias，当前终端可能还会指向旧路径。可以重新打开终端，或执行：

```sh
unalias sshm 2>/dev/null || true
hash -r
```

<details>
<summary>其他安装方式</summary>

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

```sh
sshm
```

第一次打开后，按 `a` 添加服务器。添加服务器时左边填写服务器信息，右边管理分类，`Tab` 切换左右区域，`Enter` 保存。添加完成后主面板会自动采集监控数据，选中服务器后可以直接登录、看详情、传文件或执行命令模板。

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

一键安装只安装程序本体，服务器配置会保存在用户目录里。

| 类型 | macOS / Linux | Windows |
| --- | --- | --- |
| 程序本体 | macOS Homebrew 环境默认 `/opt/homebrew/bin/sshm`，其他环境默认 `/usr/local/bin/sshm`，也可自定义安装目录 | `%LOCALAPPDATA%\Programs\sshm\sshm.exe` 或自定义安装目录 |
| 服务器配置 | `~/.config/sshm/servers.toml` | `%USERPROFILE%\.config\sshm\servers.toml` |
| 命令模板 | `~/.config/sshm/commands.toml` | `%USERPROFILE%\.config\sshm\commands.toml` |
| 命令历史 | `~/.config/sshm/history.toml` | `%USERPROFILE%\.config\sshm\history.toml` |
| 应用配置 | `~/.config/sshm/config.toml` | `%APPDATA%\sshm\config.toml` |
| 本机状态 | `~/.config/sshm/state.toml` | `%USERPROFILE%\.config\sshm\state.toml` |

打开配置目录：

```sh
open ~/.config/sshm
```

查看服务器配置：

```sh
cat ~/.config/sshm/servers.toml
```

## ⚙️ 配置

服务器数据保存在：

```text
~/.config/sshm/servers.toml
```

<details>
<summary>查看 servers.toml 示例</summary>

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
note = "线上 Web 入口"
expire_at = "2026-08-31"
favorite = true
health_ports = [80, 443, 8080]

[[servers]]
category = "staging"
name = "demo-db"
host = "198.51.100.20"
user = "admin"
port = 22
key_path = ""
proxy_jump = ""
password = "example-password"
note = "测试数据库"
expire_at = ""
favorite = false
health_ports = [5432]
```

</details>

认证方式自动判断：

- `key_path` 不为空时，只使用当前服务器配置里的密钥
- `password` 不为空时允许 `password` 和 `keyboard-interactive` / PAM
- `key_path` 和 `password` 同时存在时，先用当前配置的密钥，再用当前配置的密码
- 两者都为空时交给系统 OpenSSH、ssh-agent 或默认配置处理

连接、监控、上传下载和远程目录选择都会禁用 SSH 连接复用，避免同 IP、同用户的不同服务器配置误复用到旧连接。

可选字段：

- `favorite = true`：标记收藏服务器，面板里可按 `f` 收藏/取消收藏，按 `v` 只看收藏。
- `note = "线上 Web 入口"`：服务器备注，会显示在卡片和详情里，也可以被搜索匹配。
- `expire_at = "2026-08-31"`：服务器到期日期，格式固定为 `YYYY-MM-DD`。卡片和详情会显示剩余时间，已过期、今天到期或 7 天内到期会用颜色提示。
- `health_ports = [80, 443]`：在详情页和面板里显示健康端口状态。当前检查逻辑基于远端监听端口，端口在服务器上监听就显示正常。

应用配置保存在：

```text
~/.config/sshm/config.toml
```

<details>
<summary>查看 config.toml 示例</summary>

```toml
refresh_interval = "5s"
connect_timeout = "2s"
command_timeout = "6s"

local_dirs = [".", "~/Downloads", "~/Desktop", "~/Documents", "~"]
remote_dirs = ["$HOME", "/home", "/opt", "/var/www", "/data", "/tmp"]
```

</details>

命令模板单独保存在：

```text
~/.config/sshm/commands.toml
```

可以在 TUI 里按 `m` 添加、编辑、删除模板，也可以手动编辑这个文件。

<details>
<summary>查看 commands.toml 示例</summary>

```toml
[[global]]
name = "查看磁盘"
command = """
df -h
"""

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

</details>

命令模板分两种：

- `global`：全局模板，所有服务器都能使用。
- `server`：单服务器模板，只在对应 `分类/服务器名` 下显示。

在主面板或详情页选中服务器后，按 `m` 打开命令模板。模板列表里可以执行、新增、编辑、删除模板。
- 模板编辑里 `Enter` 保存，`Ctrl+J` 在命令内容中换行。
- 执行前会显示完整命令，确认后才会通过 SSH 在远端执行。
- 模板可以保存为全局模板，也可以保存为当前服务器专用模板；新增模板时默认使用当前服务器。
- 模板列表里提供 `+ 临时命令`，适合临时执行一次性的多行命令。

主面板按 `b` 可以进入批量命令：

- `Space` 勾选多台服务器，`a` 全选当前列表，`x` 清空选择。
- 批量命令只使用全局模板或临时命令，避免混用不同服务器的专用模板。
- 执行前会显示目标服务器和完整命令，确认后按服务器顺序串行执行。
- 结果页显示每台服务器成功、失败、等待或执行中状态，并可查看单台输出。

主面板按 `i` 可以查看命令历史：

- 单台命令、临时命令、批量命令执行完成后都会记录。
- 历史里可以查看执行时间、目标服务器、命令内容、退出码和输出。
- `r` 可以按历史里保存的命令重新执行；如果服务器已删除，会提示不能执行。
- `x` 删除当前历史记录，删除前会二次确认。
- 默认最多保留最近 100 条历史，每台服务器输出只保存最后 200 行，避免历史文件过大。

## 🔄 数据迁移

如果 `servers.toml` 不存在，首次启动会尝试从这些旧配置迁移：

```text
~/.ssh/config
~/.ssh/config.d/*
~/.ssh/passwords.txt
~/.ssh/passwords/<host>
```

迁移后，添加、编辑、删除、登录、监控、上传和下载都以 `servers.toml` 为准。保存配置时会直接覆盖 `servers.toml`，不会自动生成 `.bak` 备份文件。

## 🖥️ 登录体验

登录服务器时，`sshm` 会临时让出终端控制权，直接运行系统 `ssh` 或 `sshpass ssh`，并强制分配交互式 TTY。

这意味着进入服务器后：

- `Ctrl+C` 会中断远程命令
- 删除键、方向键、Tab 补全按远程 shell 的规则工作
- `vim`、`top`、`htop`、`tmux` 等交互式程序按正常 SSH 方式运行
- 退出远程服务器后会回到 `sshm` 面板

如果本机 OpenSSH 支持 `WarnWeakCrypto`，`sshm` 会自动隐藏下面这类 post-quantum 提示：

```text
WARNING: connection is not using a post-quantum key exchange algorithm
```

连接失败、认证失败、网络错误等正常错误仍会显示。

## 📡 监控方式

`sshm` 通过 SSH 执行远程只读命令采集 Linux 服务器状态，不安装 agent，不修改服务器配置。

| 默认策略 | 值 |
| --- | --- |
| 自动刷新 | 每 5 秒一轮 |
| SSH 连接超时 | 2 秒 |
| 单台采集超时 | 6 秒 |
| 离线判断 | 在线服务器连续失败 2 次后显示离线 |

主面板支持卡片和分类两种视图，按 `z` 循环切换。卡片视图适合看资源概览；分类视图会显示 `全部` 和真实服务器分类，收藏、在线、异常仍通过快捷键筛选。终端宽度足够时分类在左侧；宽度小于 100 列时自动切成顶部分类条和下方服务器列表。搜索时会自动以列表结果展示，`↑↓` / `j/k` 选择结果，`Enter` 登录，`Space` 查看详情。

主面板显示短进度条，方便快速扫 CPU、内存、磁盘使用率和容量。服务器卡片会显示备注、到期剩余时间或运行时间、最近登录时间，弱化地址、负载、容器等辅助信息，并用分隔线区分资源指标和状态摘要。详情页会展示更完整的资源信息，包括 CPU 核心数和型号、内核、架构、到期时间、最近登录时间、成功/失败登录摘要、检查建议、内存可用量、Swap、根分区文件系统、磁盘可用量和 inode 使用情况。详情页按基础、资源、服务、容器、登录、风险分组，左右键或 `Tab` 切换分类，内容超出窗口高度时可以用 `↑↓` 或 `j/k` 上下滚动。

登录记录只在打开详情页时查询一次，详情页按 `r` 刷新时会重新查询；主面板自动刷新不会查询登录记录。成功登录使用 `last -n 100` 统计摘要，失败登录使用 `lastb -n 100`，如果普通用户权限不足会尝试 `sudo -n lastb -n 100`，仍不可用时显示需要 root 权限。

主面板按 `w` 可以打开异常总览，按严重和警告数量排序显示有问题的服务器。异常总览里 `f` / `Tab` 可以切换筛选，也可以直接按 `0` 全部、`1` 严重、`2` 警告、`3` 离线、`4` 资源、`5` 容器、`6` 服务、`7` 安全。`Enter` 或 `Space` 可直接进入对应详情页，并自动切到更相关的详情分类。

检查建议位于详情页底部，会基于服务器到期时间、资源使用率、健康端口、故障容器、异常服务、失败登录数量，以及 `sshd -T` 中的密码登录和 Root 登录配置给出严重、警告、提示或正常项。安全相关建议会尽量明确显示成检查项，例如 `允许密码登录：风险`、`允许root登录：风险`、`SSH端口：提示`、`失败登录来源IP过多：风险`，方便快速判断该改哪里。

服务信息会按详情分类拆开显示：

- 健康：自定义 `health_ports` 是否在远端监听，以及当前监听端口列表。
- 容器：通过 `docker ps -a` 统计总数、运行、停止、故障，并按状态列出容器名称、状态、镜像和端口。`restarting` / `dead` 计为故障，`exited` / `created` / `paused` 计为停止。
- 系统服务：通过 `systemctl --failed` 显示失败服务数量和名称。

采集内容包括 `/proc/stat`、`/proc/loadavg`、`/proc/cpuinfo`、`free`、`df`、`uname`、`uptime`、`systemctl --failed`、`docker ps -a`、`ss` 或 `netstat`。

## 📁 文件传输

| 操作 | 流程 |
| --- | --- |
| 上传 | 选择服务器 -> 按 `u` -> 左侧选择本地文件/目录 -> 右侧选择远程目录 -> `Space` 开始 |
| 下载 | 选择服务器 -> 按 `d` -> 左侧选择远程文件/目录 -> 右侧选择本地目录 -> `Space` 开始 |

底层调用系统 `scp`，支持文件和目录。传输过程中按 `q` / `Esc` / `Ctrl+C` 退出 `sshm` 时，会主动中断正在运行的上传或下载子进程。

## 💻 平台支持

| 平台 | 状态 |
| --- | --- |
| macOS | ✅ 推荐 |
| Linux | ✅ 推荐 |
| Windows Terminal + OpenSSH | 🧪 实验性 |

Windows 目前可编译运行，但本地路径选择和 `sshpass` 体验没有 macOS/Linux 完整。

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
