<h1 align="center">sshm</h1>

<p align="center">
  <strong>全中文终端 SSH 服务器管理器</strong>
  <br>
  一个 TUI 面板：看监控、进 SSH、传文件、跑命令。
</p>

<p align="center">
  <a href="https://github.com/YaMaiDay/sshm/releases"><img alt="Release" src="https://img.shields.io/github/v/release/YaMaiDay/sshm?style=for-the-badge"></a>
  <a href="https://github.com/YaMaiDay/sshm/actions/workflows/release.yml"><img alt="Release Build" src="https://img.shields.io/github/actions/workflow/status/YaMaiDay/sshm/release.yml?label=release&style=for-the-badge"></a>
  <a href="https://github.com/YaMaiDay/sshm/actions/workflows/codeql.yml"><img alt="CodeQL" src="https://img.shields.io/github/actions/workflow/status/YaMaiDay/sshm/codeql.yml?label=codeql&style=for-the-badge"></a>
  <a href="https://github.com/YaMaiDay/sshm"><img alt="Go" src="https://img.shields.io/badge/Go-1.24-00ADD8?style=for-the-badge&logo=go&logoColor=white"></a>
  <a href="#-安装"><img alt="Platform" src="https://img.shields.io/badge/macOS%20%7C%20Linux%20%7C%20Windows-支持-2ea44f?style=for-the-badge"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue?style=for-the-badge"></a>
</p>

<p align="center">
  <a href="#-安装">安装</a> ·
  <a href="#-功能">功能</a> ·
  <a href="https://github.com/YaMaiDay/sshm/wiki">文档</a> ·
  <a href="https://github.com/YaMaiDay/sshm/releases">下载</a>
</p>

<p align="center">
  <img src="assets/demo-v2.svg" alt="sshm demo" width="920">
</p>

---

## ⚡ 安装

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
```

安装脚本会从 GitHub Releases 下载对应系统和架构的安装包，并自动使用同版本 `checksums.txt` 校验 SHA256。

### 手动下载与校验

不想使用安装脚本时，可以到 [Releases](https://github.com/YaMaiDay/sshm/releases) 手动下载对应系统的压缩包，并下载同一版本里的 `checksums.txt`。

macOS / Linux 校验示例：

```sh
shasum -a 256 sshm_v*_darwin_arm64.tar.gz
cat checksums.txt
```

Windows PowerShell 校验示例：

```powershell
Get-FileHash .\sshm_v*_windows_amd64.zip -Algorithm SHA256
type .\checksums.txt
```

确认本地文件的 SHA256 和 `checksums.txt` 中对应文件一致后，再解压使用。Release 同时提供 `sbom.spdx.json` 依赖清单，并启用 GitHub Artifact Attestations 用于查看构建来源。

## ✨ 功能

|  |  |
| --- | --- |
| 🖥️ | 全中文 TUI，卡片/分组/分类视图，窄屏自适应 |
| 📊 | CPU、内存、磁盘挂载点、负载、Swap、inode |
| 🐳 | Docker、监听端口、健康检查、异常服务 |
| 🔐 | 调用系统 `ssh`，保留原生终端体验 |
| 🪜 | 跳板机/堡垒机连接，密钥保存在本地 |
| 🚀 | 应用部署：Git / GitHub Release、命令流水线、部署记录、回滚命令 |
| 🛡️ | 成功/失败登录摘要，SSH 风险提示 |
| 🧰 | 命令模板、批量命令、命令历史 |
| 📁 | 文件和目录上传/下载，支持多选、任务列表、进度、暂停继续 |
| 🗂️ | 分类、分类重命名、置顶、收藏、备注、到期时间、复制服务器 |
| 🔄 | 从 OpenSSH 配置迁移 |

## 🚀 常用场景

|  |  |
| --- | --- |
| 🧑‍💻 | 多台服务器：分类、置顶、收藏、搜索 |
| 📊 | 看状态：CPU、内存、磁盘挂载点、容器 |
| 🔐 | `Enter` 直接登录服务器 |
| 🧰 | `m` 命令模板，`b` 批量执行 |
| 🚀 | `g` 应用部署，支持 Git 更新或 Release 包更新 |
| 📁 | `u` / `d` 上传下载文件或目录，`y` 查看传输任务 |
| 🛡️ | 看失败登录和 SSH 风险 |

## 📁 传输任务

sshm 使用 `rsync` 进行文件传输，适合较大的文件和目录。

|  | 说明 |
| --- | --- |
| ✅ | 可一次选择多个文件或目录创建上传/下载任务 |
| 📊 | 任务卡片显示状态、方向、路径、进度、速度和错误 |
| ⏸️ | 支持暂停、中断后继续，保留半成品用于断点续传 |
| 🧾 | 保留传输历史，已完成/失败/取消记录最多保存 100 条 |
| 🧭 | 传输中可以返回首页，任务会继续运行 |
| 🚪 | 退出 sshm 时，正在运行的传输会被标记为中断 |

如果服务器配置了跳板机，上传、下载、登录、命令执行和监控采集都会复用同一套 SSH 连接逻辑，通过跳板机连接到目标服务器。

## 🚀 应用部署

sshm 的应用部署是“选择服务器 + 获取资源 + 执行命令流水线 + 记录结果”的轻量发布功能。它复用现有 SSH 和跳板机连接逻辑，不安装远程 agent。

按 `g` 进入的是全局部署应用列表。每个部署应用在配置时选择目标服务器，执行时只会连接该应用绑定的服务器。部署列表支持卡片/列表视图，`z` 切换视图，`Tab` 在有应用的分类之间切换，空分类会跳过。`Space` 查看详情，`s` 按顺序选择多个应用，`Enter` 按选择顺序串行部署。`t` 可置顶应用，`f` 可收藏应用，`v` 可只看收藏；收藏只作为标记和筛选条件，不改变排序，置顶会优先显示。列表和详情会显示每个应用最近一次是部署还是回滚、成功还是失败、以及相对时间。

支持两种获取方式：

| 获取方式 | 说明 |
| --- | --- |
| 本地拉取后上传 | sshm 在本地访问 GitHub，再用 rsync 上传到目标服务器。GitHub 凭证保存在本地 |
| 服务器拉取 | 目标服务器自己访问 GitHub。GitHub 凭证需要在目标服务器上 |

支持两种资源来源：

| 来源 | 说明 |
| --- | --- |
| Git | 在目标服务器执行 `git clone` / `git fetch` / `git pull --ff-only` |
| Release | 在目标服务器下载 GitHub Release 资源，解压到版本目录并切换 `current` |

Release 的资源字段支持两种写法：

| 写法 | 说明 |
| --- | --- |
| `app_linux_amd64.tar.gz` | 固定文件名。版本留空或填 `latest` 时，下载最新 Release 里的同名资源 |
| `freedex-trade-kernel-amd64-*` | 匹配规则。适合文件名带日期、commit 或构建号的资源 |

如果填写完整下载地址，则优先使用下载地址，不再自动拼 GitHub Release 地址。

每个应用可以配置这些阶段：

| 阶段 | 说明 |
| --- | --- |
| 更新前 | 例如停止服务、备份配置 |
| 获取资源 | 默认按资源参数生成 Git 或 Release 命令，可在部署流程里修改 |
| 更新命令 | 例如构建、安装依赖、迁移数据库 |
| 更新后 | 例如重启服务 |
| 健康检查 | 例如 curl 本地健康端口 |
| 回滚命令 | 失败后或手动回滚时执行 |

每个应用还可以配置“等待时间”。多选部署时，当前应用执行完成后，会按当前应用的等待时间暂停，再执行队列里的下一个应用。

部署和回滚是两个确认流程：`Enter` 进入部署确认页，只显示本次部署会执行的步骤和最多 50 条历史摘要；部署输出页按 `r` 会进入回滚确认页，确认后才执行回滚命令。

GitHub 凭证不会把私钥或 token 内容写进 sshm 配置。凭证参数按获取方式解释：

| 凭证类型 | 凭证参数 |
| --- | --- |
| SSH Key | 本地拉取时是本地私钥路径；服务器拉取时是目标服务器私钥路径 |
| Token | 本地拉取时是本地环境变量名；服务器拉取时是目标服务器环境变量名 |

推荐私有 Git 仓库使用最小权限 Deploy Key。Token 更适合下载私有 GitHub Release 资源。

## 📊 监控口径

sshm 不在远程服务器安装 agent。监控数据通过一次 SSH 远程命令采集。

| 项目 | 说明 |
| --- | --- |
| CPU | 读取系统 CPU 时间差计算使用率，显示核心数和型号 |
| 内存 | 读取系统内存、可用内存和 Swap |
| 磁盘 | 使用 `df -PT -B1` 读取已挂载的真实文件系统 |
| inode | 使用 `df -Pi /` 读取根文件系统 inode |

磁盘列表只显示已挂载的真实文件系统，例如 `/`、`/boot`、`/data`、`/mnt/xxx`。`tmpfs`、`devtmpfs`、`proc`、`sysfs`、`overlay` 等内存/系统/容器临时文件系统会被过滤；未挂载的分区不会显示，因为它没有可用的使用率。

首页磁盘卡片会优先展示使用率最高的真实挂载点。如果挂载点不是 `/`，会显示为 `磁盘/data`、`磁盘/mnt/xxx` 这类形式；详情页会列出每个挂载点的设备、文件系统类型、使用率、容量和可用空间。

## 🧭 使用

```text
1. 运行 sshm
2. 按 a 添加服务器
3. Enter 保存
4. 在主面板查看监控、登录、传文件、跑命令
```

常用按键：

| 按键 | 作用 |
| --- | --- |
| `a` | 添加服务器 |
| `e` | 编辑服务器 |
| `x` | 删除服务器 |
| `Tab` / `←` / `→` | 切换分类 |
| `Enter` | 登录当前服务器 |
| `u` / `d` | 上传 / 下载 |
| `m` | 命令模板 |
| `b` | 批量命令 |
| `y` | 传输任务 |

## 📦 依赖

| 命令 | 用途 |
| --- | --- |
| `ssh` | 登录、监控采集 |
| `rsync` | 上传/下载、断点续传 |
| `sshpass` | 密码登录，可选 |

macOS：

```sh
brew install hudochenkov/sshpass/sshpass
```

Debian / Ubuntu：

```sh
sudo apt install openssh-client rsync sshpass
```

远程服务器也需要 `rsync`。如果远程缺少 `rsync`，sshm 会提示用户确认是否尝试安装；没有权限时会直接提示安装失败，不会静默修改服务器。

## 📁 配置

| 文件 | 作用 |
| --- | --- |
| `~/.config/sshm/servers.toml` | 服务器 |
| `~/.config/sshm/commands.toml` | 命令模板 |
| `~/.config/sshm/history.toml` | 命令历史 |
| `~/.config/sshm/transfers.toml` | 传输任务和历史 |
| `~/.config/sshm/deployments.toml` | 应用部署配置和部署记录 |
| `~/.config/sshm/config.toml` | 应用配置 |

<details>
<summary>更多配置说明</summary>

Windows 配置目录：

```text
%USERPROFILE%\.config\sshm\servers.toml
%USERPROFILE%\.config\sshm\commands.toml
%USERPROFILE%\.config\sshm\history.toml
%APPDATA%\sshm\config.toml
```

认证逻辑：

- 有 `key_path`：只用当前服务器密钥。
- 有 `password`：允许密码和 PAM。
- 密钥和密码都有：先密钥，后密码。
- 都没有：交给系统 OpenSSH / ssh-agent。

分类说明：

- 普通分类可以新增、删除空分类和重命名。
- `跳板机` 是固定分类，用来保存堡垒机/跳板机服务器。
- `跳板机` 分类不能重命名或删除。
- 如果某台跳板机正在被其他服务器引用，它的名称和分类不能修改。

常用字段：

```toml
category = "production"
name = "demo-web"
host = "203.0.113.10"
user = "deploy"
port = 22
key_path = "~/.ssh/id_ed25519"
note = "线上 Web 入口"
expire_at = "2026-08-31"
favorite = true
pinned = true
pinned_order = 1
health_ports = [80, 443, 8080]
```

跳板机也是一台可监控的服务器，固定放在“跳板机”分类里。内部服务器只引用跳板机名称；所有密钥路径都指向本地电脑上的私钥文件，跳板机只转发 SSH 连接，不保存目标服务器密钥。

配置完成后，用户在首页选择内部服务器并按 `Enter`，会直接进入内部服务器；监控、命令模板、上传和下载也都是对内部服务器执行，不是连接到跳板机本身。

```toml
category = "跳板机"
name = "bastion-prod-01"
host = "203.0.113.10"
user = "deploy"
port = 22
key_path = "~/.ssh/bastion_key"

category = "production"
name = "internal-web"
host = "10.0.2.21"
user = "deploy"
port = 22
key_path = "~/.ssh/app_key"
jump_host_ref = "bastion-prod-01"
```

等价的连接链路：

```text
本地电脑 --SSH--> 跳板机 --SSH--> 目标服务器
```

两段连接使用的私钥文件都在本地电脑。sshm 会临时生成 OpenSSH 配置并调用系统 `ssh` / `rsync`，不会把目标服务器密钥复制到跳板机。

</details>

## 🔒 安全、隐私与联网行为

sshm 是本地运行的 SSH 管理工具，不包含遥测，不会在后台连网检查更新，也不会向项目服务器上报服务器信息。

|  | 说明 |
| --- | --- |
| 🌐 | 运行时不会自动访问 GitHub 检查更新 |
| 📊 | 不包含遥测、数据统计或崩溃上报 |
| 🛰️ | 不在后台访问 GitHub 或项目方服务器 |
| 📡 | 不上传服务器列表、IP、用户名、路径、命令历史或传输历史 |
| 🚫 | 不安装服务器 agent |
| 🧱 | 不修改远程 `sshd_config` |
| 🔑 | 不上传密钥 |
| 🪜 | 跳板机模式下，目标服务器密钥仍保存在本地 |
| 🗂️ | 不默认扫描 `/root` |
| 🔐 | 登录直接调用系统 `ssh` |
| 📁 | 文件传输直接在本机和目标服务器之间通过 `rsync` 进行 |

密码保存在本机 `servers.toml` 中，文件权限设置为 `600`。

会主动联网的场景只有：

- 用户运行 `install.sh` / `install.ps1` 安装脚本时，会访问 GitHub Releases 下载程序。
- 用户确认远程安装 `rsync` 时，会在远程服务器上调用系统包管理器。
- 用户主动连接自己的服务器、执行命令、上传或下载文件。

## 📄 执照

Apache 2.0 — 请参阅 [LICENSE](LICENSE)。

---

### ⭐ 如果这个项目对你有用，欢迎点一个 Star！⭐

[报告问题](https://github.com/YaMaiDay/sshm/issues/new) ·
[提出功能建议](https://github.com/YaMaiDay/sshm/issues/new) ·
[参与讨论](https://github.com/YaMaiDay/sshm/discussions)
