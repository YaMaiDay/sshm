<p align="center">
  <img src="assets/preview.svg" alt="sshm preview" width="920">
</p>

<h1 align="center">sshm</h1>

<p align="center">
  <strong>全中文终端 SSH 服务器管理器</strong>
  <br>
  一个 TUI 面板：看监控、进 SSH、传文件、跑命令。
</p>

<p align="center">
  <a href="https://github.com/YaMaiDay/sshm/releases"><img alt="Release" src="https://img.shields.io/github/v/release/YaMaiDay/sshm?style=for-the-badge"></a>
  <a href="https://github.com/YaMaiDay/sshm"><img alt="Go" src="https://img.shields.io/badge/Go-1.24-00ADD8?style=for-the-badge&logo=go&logoColor=white"></a>
  <a href="#-安装"><img alt="Platform" src="https://img.shields.io/badge/macOS%20%7C%20Linux%20%7C%20Windows-支持-2ea44f?style=for-the-badge"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue?style=for-the-badge"></a>
</p>

<p align="center">
  <a href="#-安装">安装</a> ·
  <a href="#-功能">功能</a> ·
  <a href="https://github.com/YaMaiDay/sshm/releases">下载</a>
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

## ✨ 功能

|  |  |
| --- | --- |
| 🖥️ | 全中文 TUI，卡片/分类视图，窄屏自适应 |
| 📊 | CPU、内存、磁盘、负载、Swap、inode |
| 🐳 | Docker、监听端口、健康检查、异常服务 |
| 🔐 | 调用系统 `ssh`，保留原生终端体验 |
| 🛡️ | 成功/失败登录摘要，SSH 风险提示 |
| 🧰 | 命令模板、批量命令、命令历史 |
| 📁 | 文件和目录上传/下载 |
| 🗂️ | 分类、收藏、备注、到期时间、复制服务器 |
| 🔄 | 从 OpenSSH 配置迁移 |

## 🧭 使用

```text
1. 运行 sshm
2. 按 a 添加服务器
3. Enter 保存
4. 在主面板查看监控、登录、传文件、跑命令
```

## 📦 依赖

| 命令 | 用途 |
| --- | --- |
| `ssh` | 登录、监控采集 |
| `scp` | 上传/下载 |
| `sshpass` | 密码登录，可选 |

macOS：

```sh
brew install hudochenkov/sshpass/sshpass
```

Debian / Ubuntu：

```sh
sudo apt install openssh-client sshpass
```

## 📁 配置

| 文件 | 作用 |
| --- | --- |
| `~/.config/sshm/servers.toml` | 服务器 |
| `~/.config/sshm/commands.toml` | 命令模板 |
| `~/.config/sshm/history.toml` | 命令历史 |
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
health_ports = [80, 443, 8080]
```

</details>

## 🔒 安全边界

|  | 说明 |
| --- | --- |
| 🚫 | 不安装服务器 agent |
| 🧱 | 不修改远程 `sshd_config` |
| 🔑 | 不上传密钥 |
| 🗂️ | 不默认扫描 `/root` |
| 🔐 | 登录直接调用系统 `ssh` |

密码保存在本机 `servers.toml` 中，文件权限设置为 `600`。

## 📄 执照

Apache 2.0 — 请参阅 [LICENSE](LICENSE)。
