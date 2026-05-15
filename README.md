<h1 align="center">sshm</h1>

<p align="center">
  <strong>A terminal SSH server manager for monitoring, login, file transfer, commands, and deployments.</strong>
  <br>
  Local-first, agentless, and built on top of OpenSSH and rsync.
</p>

<p align="center">
  <a href="https://github.com/YaMaiDay/sshm/releases"><img alt="Release" src="https://img.shields.io/github/v/release/YaMaiDay/sshm?style=for-the-badge"></a>
  <a href="https://github.com/YaMaiDay/sshm/actions/workflows/release.yml"><img alt="Release Build" src="https://img.shields.io/github/actions/workflow/status/YaMaiDay/sshm/release.yml?label=release&style=for-the-badge"></a>
  <a href="https://github.com/YaMaiDay/sshm/actions/workflows/codeql.yml"><img alt="CodeQL" src="https://img.shields.io/github/actions/workflow/status/YaMaiDay/sshm/codeql.yml?label=codeql&style=for-the-badge"></a>
  <a href="https://github.com/YaMaiDay/sshm"><img alt="Go" src="https://img.shields.io/badge/Go-1.24-00ADD8?style=for-the-badge&logo=go&logoColor=white"></a>
  <a href="#-installation"><img alt="Platform" src="https://img.shields.io/badge/macOS%20%7C%20Linux%20%7C%20Windows-supported-2ea44f?style=for-the-badge"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/badge/license-Apache--2.0-blue?style=for-the-badge"></a>
</p>

<p align="center">
  <a href="#-installation">Installation</a> ·
  <a href="#-features">Features</a> ·
  <a href="docs/troubleshooting.md">Troubleshooting</a> ·
  <a href="https://github.com/YaMaiDay/sshm/releases">Downloads</a>
</p>

<p align="center">
  <img src="assets/demo-v2.svg" alt="sshm demo" width="920">
</p>

---

## ⚡ Installation

macOS / Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.sh | sh
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.ps1 | iex
```

Run:

```sh
sshm
```

The install script downloads the matching package from GitHub Releases and verifies it with the same-version `checksums.txt`.

### Manual Download And Verification

If you do not want to use the install script, download the archive for your platform from [Releases](https://github.com/YaMaiDay/sshm/releases), then download the same release's `checksums.txt`.

macOS / Linux:

```sh
shasum -a 256 sshm_v*_darwin_arm64.tar.gz
cat checksums.txt
```

Windows PowerShell:

```powershell
Get-FileHash .\sshm_v*_windows_amd64.zip -Algorithm SHA256
type .\checksums.txt
```

After the local SHA256 matches `checksums.txt`, extract and run the binary. Releases also include `sbom.spdx.json` and GitHub Artifact Attestations for build provenance.

## ✨ Features

|  |  |
| --- | --- |
| 🖥️ | TUI dashboard with card, group, category, and narrow-screen layouts |
| 🌐 | English-first language setting, with Chinese available as a secondary language |
| 📊 | CPU, memory, mounted disks, load, swap, inode, and uptime |
| 🐳 | Docker containers, listening ports, health checks, and failed services |
| 🔐 | Uses the system `ssh`, preserving the native terminal experience |
| 🪜 | Bastion / jump host support with private keys kept on the local machine |
| 🚀 | App deployment with Git, GitHub Releases, command stages, history, and rollback |
| 🛡️ | Login summary and SSH risk hints |
| 🧰 | Command templates, batch commands, and command history |
| 📁 | File and directory upload/download with multi-select, task history, and progress |
| 🗂️ | Categories, rename, pin, favorite, notes, expiration dates, and server copy |
| 🔄 | Migration from OpenSSH config |

## 🚀 Common Workflows

|  |  |
| --- | --- |
| 🧑‍💻 | Manage many servers with categories, pins, favorites, and search |
| 📊 | Check CPU, memory, mounted disks, containers, and basic health |
| 🔐 | Press `Enter` to open SSH for the selected server |
| 🧰 | Press `m` for command templates and `b` for batch commands |
| 🚀 | Press `g` for Git or GitHub Release based deployment |
| 📁 | Press `u` / `d` to upload or download files and `y` to view transfer jobs |
| 🛡️ | Inspect failed logins and SSH risk information |

## 📁 File Transfers

sshm uses `rsync` for file transfers, which works well for large files and directories.

|  | Description |
| --- | --- |
| ✅ | Create upload/download jobs from multiple selected files or directories |
| 📊 | Transfer cards show status, direction, paths, progress, speed, and errors |
| ⏸️ | Pause and resume interrupted jobs while preserving partial files |
| 🧾 | Keep up to 100 completed, failed, or canceled transfer records |
| 🧭 | Return to the dashboard while transfers continue in the background |
| 🚪 | Running transfers are marked interrupted when sshm exits |

If a server uses a bastion host, upload, download, login, command execution, and monitoring all reuse the same SSH connection logic through that bastion.

## 🚀 App Deployment

sshm deployment is a lightweight release workflow: select a server, fetch a Git or GitHub Release resource, run command stages, record the result, and optionally roll back. It reuses the existing SSH, bastion, and rsync connection logic and does not install a remote agent.

Press `g` to open the global deployment app list. Each app is bound to one target server; running that app only connects to its configured server. The list supports card/list views, category filtering, pins, favorites, multi-select queues, and last deploy/rollback status.

Fetch modes:

| Fetch Mode | Description |
| --- | --- |
| Local fetch then upload | sshm accesses GitHub locally, then uploads the resource to the target server with rsync. GitHub credentials stay local |
| Remote fetch | The target server accesses GitHub directly. GitHub credentials must exist on that server |

Resource sources:

| Source | Description |
| --- | --- |
| Git | Run `git clone`, `git fetch`, and `git pull --ff-only` |
| Release | Download a GitHub Release asset, extract it into a version directory, and switch `current` |

Release asset matching:

| Value | Description |
| --- | --- |
| `app_linux_amd64.tar.gz` | Exact asset name. With an empty version or `latest`, sshm downloads this asset from the latest Release |
| `freedex-trade-kernel-amd64-*` | Wildcard match for assets with dates, commits, or build numbers in the filename |

If a full download URL is configured, sshm uses it directly instead of building a GitHub Release URL.

Deployment stages:

| Stage | Description |
| --- | --- |
| Pre-update | Stop services, back up files, or prepare directories |
| Fetch resource | Generated from source/fetch/repo/version/asset/path/credential settings, or customized in the app |
| Update | Build, install dependencies, migrate databases, or switch services |
| Post-update | Restart services or clean temporary files |
| Health check | Run checks such as `curl http://127.0.0.1:8080/health` |
| Rollback | Separate rollback command stage, executed only from the rollback flow |

Deploy and rollback are separate confirmation flows. `Enter` opens deploy confirmation and only shows the deploy stages. After a deployment output page is open, `r` enters rollback confirmation and runs only the rollback commands.

GitHub credentials are stored as references only; sshm does not store private key contents or token values.

| Credential Type | Credential Parameter |
| --- | --- |
| SSH Key | Local fetch: local private key path. Remote fetch: target-server private key path |
| Token | Local fetch: local environment variable name. Remote fetch: target-server environment variable name |

For private Git repositories, prefer a least-privilege GitHub Deploy Key. For private Release assets, use a minimal token.

See [Deployment](docs/deployment.md) for the full workflow.

## 📊 Monitoring Model

sshm does not install an agent on remote servers. Monitoring data is collected by running one SSH command on the target server.

| Metric | Source |
| --- | --- |
| CPU | CPU time delta, core count, and model |
| Memory | System memory, available memory, and swap |
| Disk | Mounted real filesystems from `df -PT -B1` |
| inode | Root filesystem inode data from `df -Pi /` |

Disk data is based on mounted filesystems, not raw block device names. sshm shows real mount points such as `/`, `/boot`, `/data`, and `/mnt/xxx`, and filters temporary or system filesystems such as `tmpfs`, `devtmpfs`, `proc`, `sysfs`, and `overlay`. Unmounted partitions are not shown because they have no filesystem usage percentage.

The dashboard disk card highlights the real mounted filesystem with the highest usage. If that mount point is not `/`, it is shown as labels such as `disk/data` or `disk/mnt/xxx`. The detail page lists mount point, device, filesystem type, usage, capacity, and available space.

## 🧭 Usage

```text
1. Run sshm
2. Press a to add a server
3. Press Enter to save
4. Use the dashboard to monitor, log in, transfer files, and run commands
```

Common keys:

| Key | Action |
| --- | --- |
| `a` | Add server |
| `e` | Edit server |
| `x` | Delete server |
| `Tab` / `←` / `→` | Switch category |
| `Enter` | SSH into selected server |
| `u` / `d` | Upload / download |
| `m` | Command templates |
| `b` | Batch commands |
| `y` | Transfer jobs |
| `g` | App deployments |
| `.` | Settings |

## 📚 Documentation

| Document | Description |
| --- | --- |
| [Troubleshooting](docs/troubleshooting.md) | SSH, bastion hosts, rsync, GitHub, disks, and container status |
| [Security Policy](SECURITY.md) | Security boundaries, sensitive data, and local files |
| [Changelog](CHANGELOG.md) | User-visible changes |

## 📦 Dependencies

| Command | Purpose |
| --- | --- |
| `ssh` | Login and monitoring collection |
| `rsync` | Upload/download and resumable transfers |
| `sshpass` | Optional password-login automation |

macOS:

```sh
brew install hudochenkov/sshpass/sshpass
```

Debian / Ubuntu:

```sh
sudo apt install openssh-client rsync sshpass
```

Remote servers also need `rsync` for file transfer and local-fetch deployment. If a remote server is missing `rsync`, sshm asks before attempting installation. Without permission, installation fails explicitly.

## 📁 Configuration

| File | Purpose |
| --- | --- |
| `~/.config/sshm/servers.toml` | Servers |
| `~/.config/sshm/commands.toml` | Command templates |
| `~/.config/sshm/history.toml` | Command history |
| `~/.config/sshm/transfers.toml` | Transfer jobs and history |
| `~/.config/sshm/deployments.toml` | Deployment apps and records |
| `~/.config/sshm/config.toml` | App settings |

Press `.` on the dashboard to open settings. Common settings can be edited in the TUI without manually editing `config.toml`.

| Setting | Description |
| --- | --- |
| Language | `en` / `zh`. Default is English |
| ASCII mode | Reserved for terminals that need plain ASCII display |
| Refresh interval | Dashboard collection interval, for example `5s`, `30s`, or `1m` |
| Connect timeout | SSH connection timeout, for example `2s` or `5s` |
| Command timeout | Monitoring and remote-command timeout, for example `6s` or `30s` |
| CPU / memory / disk thresholds | Warning and critical colors used by dashboard and detail pages |
| Local roots | Local shortcut directories used by the transfer picker, comma-separated |
| Remote roots | Remote shortcut directories used by the transfer picker, comma-separated |

<details>
<summary>More Configuration Details</summary>

Windows config paths:

```text
%USERPROFILE%\.config\sshm\servers.toml
%USERPROFILE%\.config\sshm\commands.toml
%USERPROFILE%\.config\sshm\history.toml
%APPDATA%\sshm\config.toml
```

Authentication behavior:

- With `key_path`: use the server's configured key.
- With `password`: allow password/PAM login through `sshpass`.
- With both key and password: try key first, then password.
- With neither: let OpenSSH / ssh-agent handle authentication.

Category behavior:

- Normal categories can be created, renamed, and deleted when empty.
- `Bastion` is a fixed category used for bastion / jump hosts.
- The fixed bastion category cannot be renamed or deleted.
- If a bastion server is referenced by another server, its name and category cannot be changed.

Common fields:

```toml
category = "production"
name = "demo-web"
host = "203.0.113.10"
user = "deploy"
port = 22
key_path = "~/.ssh/id_ed25519"
note = "Production Web entry"
expire_at = "2026-08-31"
favorite = true
pinned = true
pinned_order = 1
health_ports = [80, 443, 8080]
```

A bastion is also a monitorable server. Internal servers reference the bastion by name. All private-key paths point to local private-key files; the bastion only forwards SSH connections and does not store target-server private keys.

After configuration, selecting an internal server and pressing `Enter` opens SSH directly to that internal server. Monitoring, command templates, upload, and download also operate on the internal server, not on the bastion itself.

```toml
category = "Bastion"
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

Equivalent connection path:

```text
local machine --SSH--> bastion --SSH--> target server
```

Both private keys stay on the local machine. sshm creates a temporary OpenSSH config and calls system `ssh` / `rsync`; it never copies target-server keys to the bastion.

</details>

## 🔒 Security, Privacy, And Network Behavior

sshm is a local SSH management tool. It has no telemetry, does not check for updates in the background, and does not report server data to project infrastructure.

|  | Description |
| --- | --- |
| 🌐 | No runtime GitHub update checks |
| 📊 | No telemetry, analytics, or crash reporting |
| 🛰️ | No background calls to GitHub or project servers |
| 📡 | No upload of server lists, IPs, usernames, paths, command history, or transfer history |
| 🚫 | No remote agent installation |
| 🧱 | Does not modify remote `sshd_config` |
| 🔑 | Does not upload private keys |
| 🪜 | In bastion mode, target-server keys still stay local |
| 🗂️ | Does not scan `/root` by default |
| 🔐 | Login directly calls the system `ssh` |
| 📁 | File transfer runs directly between the local machine and target server through `rsync` |

Passwords are stored locally in `servers.toml` with file permissions set to `600`.

Network access only happens when:

- The user runs `install.sh` / `install.ps1`, which downloads from GitHub Releases.
- The user confirms remote `rsync` installation, which invokes the target server's package manager.
- The user connects to their own servers, runs commands, uploads files, or downloads files.

## 📄 License

Apache 2.0. See [LICENSE](LICENSE).

---

### ⭐ If sshm is useful to you, a star is appreciated.

[Report a bug](https://github.com/YaMaiDay/sshm/issues/new) ·
[Request a feature](https://github.com/YaMaiDay/sshm/issues/new) ·
[Join discussions](https://github.com/YaMaiDay/sshm/discussions)
