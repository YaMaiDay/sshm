# v0.1.12

## 修复

- 密码登录同时允许 `password` 和 `keyboard-interactive` / PAM，兼容更多服务器的密码认证方式。
- 当服务器同时配置 `key_path` 和 `password` 时，不再禁用公钥认证，避免 AWS 等只接受公钥的服务器被密码分支误伤。
- 退出 SSH 会回到 `sshm` 面板，并用状态栏显示登录退出结果。
- 上传或下载过程中退出 `sshm` 时，会主动中断正在运行的 `scp` / `sshpass` 子进程。

## 安装

macOS / Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.sh | sh
```

指定版本:

```sh
curl -fsSL https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.sh | SSHM_VERSION=v0.1.12 sh
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.ps1 | iex
```

Windows 指定版本:

```powershell
$env:SSHM_VERSION="v0.1.12"; irm https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.ps1 | iex
```

GitHub Actions 会在推送 `v0.1.12` tag 后自动生成以下安装包和校验文件：

- `sshm_v0.1.12_darwin_amd64.tar.gz`
- `sshm_v0.1.12_darwin_arm64.tar.gz`
- `sshm_v0.1.12_linux_amd64.tar.gz`
- `sshm_v0.1.12_linux_arm64.tar.gz`
- `sshm_v0.1.12_windows_amd64.zip`
- `sshm_v0.1.12_windows_arm64.zip`
- `checksums.txt`
