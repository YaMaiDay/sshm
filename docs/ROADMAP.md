# 开发路线

## 阶段 1：项目基础

- 初始化 Go module
- 建立项目目录
- 加入 TUI 依赖
- 建立基础中文界面
- 读取应用配置

## 阶段 2：服务器配置管理

- 使用 `~/.config/sshm/servers.toml`
- 支持从旧 OpenSSH 配置迁移
- 解析服务器名称、地址、用户、端口、密钥、密码、分类
- 添加服务器
- 编辑服务器
- 删除服务器
- 移动分类
- 写入前自动备份

## 阶段 3：监控 Dashboard

- 并发 worker
- 在线/离线检测
- CPU、内存、磁盘、Load、Uptime
- 卡片式中文监控界面
- 颜色阈值
- 自动刷新和手动刷新
- 详情页

## 阶段 4：登录与文件传输

- Enter 调用系统 SSH
- 上传文件/目录
- 下载文件/目录
- 本地目录选择
- 远程目录选择
- 传输结果提示

## 阶段 5：搜索、筛选、排序

- 搜索名称/IP/用户/分类
- 分类筛选
- 在线筛选
- 异常筛选
- CPU/MEM/DISK 排序

## 阶段 6：跨平台发布

- macOS arm64/amd64
- Linux amd64/arm64
- Windows amd64
- GitHub Releases
- Homebrew tap
- Scoop/Winget 规划
