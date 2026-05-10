# sshm

全中文终端 TUI SSH 管理器。

`sshm` 目标是把 SSH 服务器管理、实时监控、登录和文件传输放到一个终端界面里。服务器数据统一保存在 `~/.config/sshm/servers.toml`，首次启动可从已有 OpenSSH 配置迁移。

## 定位

- 全中文界面
- 终端图形化监控面板
- 统一管理服务器配置
- 支持添加、编辑、删除服务器
- 支持 SSH 登录
- 支持文件/目录上传下载
- 不需要服务器安装 agent
- 尽量支持 macOS、Linux、Windows

## 设计原则

- 主机数据保存在：`~/.config/sshm/servers.toml`
- 工具行为配置保存在：`~/.config/sshm/config.toml`
- 首次启动时兼容迁移 `~/.ssh/config`、`~/.ssh/config.d/*` 和旧密码文件
- 默认不内置远程 shell，登录时调用系统 `ssh`，避免抢占 Tab 补全
- 文件传输调用系统 `scp` / `scp -r`
- 监控通过 SSH 执行远程只读命令，不修改服务器
- 修改服务器配置前自动备份

## 计划功能

- 图形化监控 Dashboard
- 服务器详情页
- 添加、编辑、删除、移动服务器
- 密码和密钥统一配置
- SSH 登录
- 文件和目录上传
- 文件和目录下载
- 搜索、筛选、排序
- 全中文帮助和快捷键
- 跨平台发布

## 推荐运行环境

- macOS: Terminal、iTerm2、Ghostty
- Linux: GNOME Terminal、Konsole、Alacritty、Ghostty
- Windows: Windows Terminal + OpenSSH

## 安装

一键安装 macOS / Linux 最新版本：

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

依赖说明：

- 登录和采集依赖系统 `ssh`
- 上传下载依赖系统 `scp`
- 密码登录建议安装 `sshpass`

## 文档

- [产品规划](docs/PRODUCT_SPEC.md)
- [技术设计](docs/TECHNICAL_DESIGN.md)
- [配置与数据](docs/CONFIG_AND_DATA.md)
- [安全边界](docs/SECURITY.md)
- [开发路线](docs/ROADMAP.md)

## 当前开发状态

已实现：

- 读取和管理 `~/.config/sshm/servers.toml`
- 解析服务器名称、地址、用户、端口、密钥、跳板机、密码和分类
- 首次启动可从 `~/.ssh/config`、`~/.ssh/config.d/*`、`~/.ssh/passwords.txt` 迁移
- 全中文 TUI 监控面板
- 服务器卡片、CPU/内存/磁盘进度条
- 并发 SSH 采集 Linux 服务器指标
- 搜索、移动、刷新、退出
- Enter 调用系统 `ssh` 登录，密码主机优先使用 `sshpass`
- 空格打开服务器详情页
- `u` 上传文件/目录
- `d` 下载文件/目录
- 本地/远程文件选择列表
- 远程目录诊断命令 `--remote-dirs`
- `a` 添加服务器，写入 `servers.toml`
- `e` 编辑服务器，支持修改服务器名称、地址、用户、端口、密钥、跳板机、密码和分类
- `x` 删除服务器
- 修改服务器配置前自动生成 `.bak.YYYYMMDD-HHMMSS` 备份
- `s` 切换排序：默认、状态、CPU、内存、磁盘
- `o` 只看在线，再按一次取消
- `p` 只看异常，再按一次取消
- `t` 按分类循环筛选
- 读取应用配置文件 `~/.config/sshm/config.toml`
- GitHub Release 自动构建 macOS、Linux、Windows 二进制

## 本地运行

列出解析到的服务器：

```sh
go run ./cmd/sshm --list
```

采集单台服务器指标：

```sh
go run ./cmd/sshm --probe demo-web
```

列出单台服务器可选择的远程目录：

```sh
go run ./cmd/sshm --remote-dirs demo-web
```

查看配置文件路径：

```sh
go run ./cmd/sshm --config-path
```

启动 TUI：

```sh
go run ./cmd/sshm
```

或先构建：

```sh
go build ./cmd/sshm
./sshm
```
