# 安全边界

## 默认行为

默认只读服务器状态，不修改服务器。

监控只执行远程只读命令：

- `/proc/stat`
- `/proc/loadavg`
- `free`
- `df`
- `uptime`
- `systemctl --failed`
- `docker ps`
- `ss` 或 `netstat`

## 不做的事

- 不安装服务器 agent
- 不修改远程 `sshd_config`
- 不自动关闭密码登录
- 不上传密钥
- 不默认扫描 `/root`
- 不内置远程 shell

## 密码处理

密码保存在本机 `servers.toml` 中，文件权限设置为 `600`。这是为了个人终端工具使用方便，不等同于加密保险箱。

配置文件：

```text
~/.config/sshm/servers.toml
```

使用密码时：

1. 创建临时文件
2. `chmod 600`
3. 调用 `sshpass -f`
4. 用完立即删除

Windows 上默认不保证支持 `sshpass`。

## 配置写入

修改服务器配置前必须：

- 显示将要修改的文件
- 自动备份
- 删除服务器时二次确认

## 命令执行

本地命令使用 `exec.Command` 参数数组，不拼接 shell 字符串。

远程命令只使用固定内置脚本，用户自定义命令功能需要明确确认。
