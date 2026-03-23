# 天枢Runner - 阿里云云效兼容的流水线执行引擎

功能特性:
  - 支持本地执行(exec)和Docker容器执行
  - 兼容阿里云云效Flow的YAML格式
  - 多阶段、多任务、多步骤的流水线
  - 环境变量和变量替换
  - 条件执行和错误处理
  - 实时上报Runner资源使用情况（CPU、内存、磁盘）
  - 支持守护进程模式，可持续在后台运行并发送心跳
  - 支持多种凭证类型进行代码克隆（SSH密钥、Token、用户名/密码）

## 使用方法

```bash
# 自动选择执行引擎
dubihe run -f pipeline.yaml

# 使用 Docker 执行
dubihe run -f pipeline.yaml -e docker

# 使用本地执行
dubihe run -f pipeline.yaml -e exec

# 以守护进程模式运行，在后台持续发送心跳并拉取执行任务
dubihe daemon

# 验证配置文件
dubihe validate -f pipeline.yaml

# 解析配置文件
dubihe parse -f pipeline.yaml
```

守护进程模式:
  使用 dubihe daemon 命令可以让 Runner 在前台以守护模式运行，
  持续向服务器发送心跳并接收任务。

  也可以使用 scripts/dubihe-daemon.sh 脚本来实现后台运行:
    ./scripts/dubihe-daemon.sh start   # 启动后台守护进程
    ./scripts/dubihe-daemon.sh stop    # 停止后台守护进程
    ./scripts/dubihe-daemon.sh status  # 查看运行状态

  或者使用 systemd 服务 (适用于 Linux 系统):
    1. 将 dubihe-runner 二进制文件复制到 /opt/dubihe-runner/
    2. 复制 systemd/dubihe-runner.service 到 /etc/systemd/system/
    3. 执行 systemctl daemon-reload
    4. 执行 systemctl enable dubihe-runner
    5. 执行 systemctl start dubihe-runner

## 安装

Dubihe Runner 可以通过以下方式安装：

### 1. 使用 Makefile 安装（推荐）

```bash
make simulate-install
```

### 2. 使用安装脚本

```bash
./scripts/install.sh -v 1.0.0 -t your-tenant-id -a your-token -w https://your-endpoint.com
```

### 3. 手动安装

参见 [README_INSTALL.md](README_INSTALL.md) 文件了解详细的手动安装步骤。

---

**安装完成后的位置：**

Runner 将位于 `~/.yunxiao-runner/{version}/runner`，并且包含 LICENSE 文件和 license 文件夹。

## 凭证类型支持

Dubihe Runner 支持以下三种凭证类型进行代码克隆：

1. **SSH密钥认证** - 适用于Git SSH地址（如 `git@github.com:user/repo.git`）
   - 在pipeline配置中设置 `certificate.type: ssh`
   - Runner会自动使用提供的私钥进行认证

2. **Token认证** - 适用于HTTPS地址和访问令牌
   - 在pipeline配置中设置 `certificate.type: token`
   - 支持GitHub和GitLab等平台的个人访问令牌

3. **用户名/密码认证** - 适用于HTTPS地址和账号密码
   - 在pipeline配置中设置 `certificate.type: password`
   - 注意：由于安全原因，建议优先使用Token认证

## 日志输出

### 日志输出位置

| 模式 | 输出位置 |
|------|----------|
| 开发模式 | 控制台输出（使用彩色日志格式） |
| 守护进程模式 | `/var/log/dubihe-runner.log` 或当前目录下的 `dubihe-runner.log` |

**自定义日志文件路径：**
```bash
dubihe run -f pipeline.yaml --log-file /path/to/your/logfile.log
```

### 日志格式

日志格式采用年月日时分秒毫米时间戳格式：

```
2025-12-19 17:22:28.000 INFO  [component] 日志消息
```

**格式说明：**

- **时间戳格式：** `YYYY-MM-DD HH:MM:SS.sss`
- **日志级别：** DEBUG, INFO, WARN, ERROR
- **组件标识：** `[component]` 标识产生日志的组件
- **日志消息：** 包含具体的操作信息和相关参数

## 示例

运行示例流水线：

```bash
# 使用本地执行引擎运行示例
dubihe run -f ci-build-push-image.yaml -e exec

run -f ci-build-push-image.yaml -e exec
```