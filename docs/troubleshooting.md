# 排错

这份文档按常见现象整理。提交 Issue 前，请先移除密码、私钥、token、私有 IP 和生产主机名。

## 安装后版本不是最新

运行：

```sh
sshm --version
```

安装脚本默认下载 GitHub Releases 的 latest 版本。如果仓库代码已经更新，但还没有创建新的 Release，安装脚本仍会安装上一个 Release。

可以指定版本：

```sh
SSHM_VERSION=v0.1.40 curl -fsSL https://raw.githubusercontent.com/YaMaiDay/sshm/main/install.sh | sh
```

也可以手动到 Releases 下载对应系统和架构的包，并用同版本的 `checksums.txt` 校验。

## SSH 连接失败

先确认系统 ssh 能不能连接：

```sh
ssh -p 22 user@example.com
```

常见原因：

| 现象 | 可能原因 |
| --- | --- |
| `Permission denied (publickey)` | 密钥路径不对、服务器没有对应公钥、账号不对 |
| `Connection timed out` | IP、端口、防火墙、安全组不通 |
| `No route to host` | 网络路由不通 |
| `Host key verification failed` | known_hosts 冲突 |
| 密码登录失败 | 服务器禁用了密码登录，或没有安装 `sshpass` |

sshm 调用系统 `ssh`，不会绕过 OpenSSH 的认证规则。

## 跳板机连接失败

跳板机链路是：

```text
本地电脑 --SSH--> 跳板机 --SSH--> 目标服务器
```

需要同时满足：

- 本地电脑能 SSH 到跳板机。
- 跳板机能访问目标服务器 SSH 端口。
- 本地电脑有跳板机私钥和目标服务器私钥，或 ssh-agent 已经加载对应密钥。
- 目标服务器的安全组或防火墙允许跳板机访问。

sshm 不会把目标服务器密钥复制到跳板机。目标服务器密钥路径仍然是本地电脑上的路径。

## 监控显示 0 或离线

sshm 通过 SSH 执行远程命令采集监控。如果 SSH 不通，监控就会失败。

还需要确认远程系统有常见命令：

```sh
uname
cat
df
awk
sed
free
ps
```

如果某些指标缺失，通常是远程系统命令缺失、权限不足或输出格式与常见 Linux 不一致。

## 磁盘为什么显示挂载点，不显示 sda1

Linux 上真正有使用率的是“文件系统挂载点”，不是裸设备名。

例如：

```text
/      设备 /dev/mapper/cs-root
/boot  设备 /dev/sda1
/data  设备 /dev/sdb1
```

`/dev/sdb3` 如果没有挂载，就没有可用容量和使用率，sshm 不会显示。

sshm 使用 `df -PT -B1` 读取已挂载的真实文件系统，并过滤 `tmpfs`、`devtmpfs`、`proc`、`sysfs`、`overlay` 等临时或系统文件系统。

## 内存和 Proxmox 显示不一致

不同系统对“已用内存”的口径可能不同。

sshm 更接近 Linux `free` 的口径，主要看可用内存：

```text
used = total - available
```

Proxmox 可能包含或排除缓存、宿主机统计、虚拟化层统计。判断服务器是否真的内存紧张，优先看 available、swap 使用率和业务进程状态。

## rsync 不存在

文件传输和“本地拉取后上传”的应用部署依赖 rsync。

本地需要安装 rsync，远程服务器也需要安装 rsync。

Debian / Ubuntu：

```sh
sudo apt install rsync
```

RHEL / CentOS / Rocky：

```sh
sudo yum install rsync
```

macOS：

```sh
brew install rsync
```

如果远程缺少 rsync，sshm 会提示是否尝试安装。没有 sudo 权限时，需要手动安装。

## GitHub 拉取失败

常见错误：

| 错误 | 说明 |
| --- | --- |
| `Permission denied (publickey)` | SSH Key 没有权限访问仓库 |
| `Repository not found` | 仓库地址错误，或 token/密钥没有权限 |
| `Could not resolve host: github.com` | 当前执行拉取的一端无法访问 GitHub |
| `HTTP 401/403` | Token 无效、过期或权限不足 |

先判断“获取方式”：

- 本地拉取后上传：检查本地电脑能否访问 GitHub，本地凭证是否正确。
- 服务器拉取：检查目标服务器能否访问 GitHub，目标服务器凭证是否正确。

SSH 仓库地址示例：

```text
git@github.com:owner/repo.git
```

Release 仓库字段示例：

```text
owner/repo
```

## Release 资源找不到

如果资源文件写固定文件名，GitHub Release 中必须存在同名资源。

如果文件名每次带日期或构建号，可以使用 `*`：

```text
freedex-trade-kernel-amd64-*
```

版本留空或填 `latest` 表示最新 Release。填 `v1.2.3` 表示固定 tag 对应的 Release。

如果填写完整下载地址，sshm 会优先使用下载地址，不再自动拼 Release 地址。

## 部署队列失败后怎么办

队列中任意一个应用失败，后续应用不会继续执行。

在确认部署页：

- 按 `r` 重试失败项。
- 按 `a` 从第一个应用重新部署。
- 按 `Esc` 返回部署列表。

失败时先看当前阶段输出。常见失败点是 GitHub 凭证、rsync、更新命令或健康检查。

## Docker 容器显示异常是什么意思

sshm 会把 Docker 的原始状态摘要成中文状态，同时在详情里显示原始状态。

常见状态：

| sshm 状态 | Docker 原始状态示例 | 含义 |
| --- | --- | --- |
| 运行 | `Up 2 weeks` | 容器正在运行 |
| 异常 | `Up 2 weeks (unhealthy)` | 容器运行中，但健康检查失败 |
| 重启中 | `Restarting (1) 10 seconds ago` | 容器正在反复重启 |
| 停止 | `Exited (0) 2 hours ago` | 容器已退出 |

如果需要进一步排查，在目标服务器执行：

```sh
docker ps -a
docker logs <container>
docker inspect <container>
```

## 提交 Issue 时需要什么

请提供：

- sshm 版本：`sshm --version`
- 操作系统和 CPU 架构
- 终端程序
- 复现步骤
- 期望行为
- 实际行为
- 已脱敏的截图或输出

不要提供：

- 密码
- 私钥
- token
- 私有 IP
- 生产主机名
- 完整服务器列表
