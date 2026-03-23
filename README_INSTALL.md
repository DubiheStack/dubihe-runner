# Dubihe Runner 安装目录结构说明

本文档描述了 Dubihe Runner 安装后生成的目录结构，参考了阿里云效 Runner 的安装目录结构。

## 安装后的目录结构

安装完成后，将在用户主目录下创建 `.dubihe-runner` 目录，其结构如下：

```
~/.dubihe-runner/
├── 1.0.0/                          # 版本目录
│   ├── runner                       # Runner 可执行文件
│   ├── LICENSE                      # 项目许可证文件
│   ├── config/                      # 配置文件目录
│   │   └── config.yaml              # 默认配置文件
│   ├── workspace/                   # 工作空间目录
│   ├── license/                     # 许可证文件目录
│   │   └── license                  # 示例许可证文件
│   └── externals/                   # 外部依赖目录
│       └── node20/                  # Node.js 20 环境
│           ├── LICENSE              # Node.js 许可证文件
│           ├── bin/                 # 可执行文件目录
│           │   ├── node             # Node.js 可执行文件
│           │   ├── npm              # NPM 包管理器
│           │   ├── npx              # NPX 包执行器
│           │   └── corepack         # Corepack 工具
│           └── lib/                 # 库文件目录
│               └── node_modules/    # Node 模块目录
│                   └── corepack/    # Corepack 模块
│                       └── shims/   # Shim 文件目录
```

## 安装步骤

1. 编译项目：
   ```bash
   go build -o dubihe-runner .
   ```

2. 创建安装目录结构：
   ```bash
   make simulate-install
   ```

   或手动创建：
   ```bash
   # 设置版本号
   VERSION=1.0.0
   INSTALL_PATH=~/.dubihe-runner
   
   # 创建目录结构
   mkdir -p $INSTALL_PATH/$VERSION/{config,workspace,externals/node20/{bin,lib/node_modules/corepack/shims}}
   
   # 复制可执行文件
   cp dubihe-runner $INSTALL_PATH/$VERSION/runner
   
   # 复制配置文件
   cp config.yaml $INSTALL_PATH/$VERSION/config/config.yaml
   
   # 创建 LICENSE 文件
   echo "Copyright (c) 2025 DubiheStack. All rights reserved." > $INSTALL_PATH/$VERSION/LICENSE
   echo "" >> $INSTALL_PATH/$VERSION/LICENSE
   echo "Licensed under the Apache License, Version 2.0 (the \"License\");" >> $INSTALL_PATH/$VERSION/LICENSE
   echo "you may not use this file except in compliance with the License." >> $INSTALL_PATH/$VERSION/LICENSE
   echo "You may obtain a copy of the License at" >> $INSTALL_PATH/$VERSION/LICENSE
   echo "" >> $INSTALL_PATH/$VERSION/LICENSE
   echo "    http://www.apache.org/licenses/LICENSE-2.0" >> $INSTALL_PATH/$VERSION/LICENSE
   echo "" >> $INSTALL_PATH/$VERSION/LICENSE
   echo "Unless required by applicable law or agreed to in writing, software" >> $INSTALL_PATH/$VERSION/LICENSE
   echo "distributed under the License is distributed on an \"AS IS\" BASIS," >> $INSTALL_PATH/$VERSION/LICENSE
   echo "WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied." >> $INSTALL_PATH/$VERSION/LICENSE
   echo "See the License for the specific language governing permissions and" >> $INSTALL_PATH/$VERSION/LICENSE
   echo "limitations under the License." >> $INSTALL_PATH/$VERSION/LICENSE
   
   # 创建空的 Node.js 文件（实际安装时会包含真正的二进制文件）
   touch $INSTALL_PATH/$VERSION/externals/node20/{LICENSE,bin/node,bin/npm,bin/npx,bin/corepack}
   
   # 创建license文件夹和示例license文件
   mkdir -p $INSTALL_PATH/$VERSION/license
   echo "# 示例许可证文件" > $INSTALL_PATH/$VERSION/license/license
   echo "" >> $INSTALL_PATH/$VERSION/license/license
   echo "这是一个示例许可证文件。" >> $INSTALL_PATH/$VERSION/license/license
   echo "请根据实际需要替换为正式的许可证文件。" >> $INSTALL_PATH/$VERSION/license/license
   ```

3. 验证安装：
   ```bash
   ls -la ~/.dubihe-runner/1.0.0/
   ```

## 配置文件说明

安装目录中的 `config.yaml` 文件包含了 Runner 的默认配置：

- `uid`: Runner 唯一标识符
- `url`: 服务端地址
- `token`: 认证令牌
- `tenant`: 租户信息
- `type`: Runner 类型
- `version`: 版本号
- `heartbeatInterval`: 心跳间隔时间
- `scanInterval`: 任务扫描间隔时间
- `concurrency`: 并发任务数
- `autoUpgrade`: 是否自动升级
- `upgradeInterval`: 升级检查间隔
- `workDir`: 工作目录
- `logConfig`: 日志配置
- `logFile`: 日志文件路径
- `devMode`: 开发模式开关

## 使用说明

安装完成后，可以通过以下方式运行 Runner：

```bash
# 进入安装目录
cd ~/.dubihe-runner/1.0.0/

# 运行 Runner
./runner --help

# 以守护进程模式运行
./runner daemon

# 运行流水线
./runner run -f /path/to/pipeline.yaml
```

## 打包发布

要创建可用于分发的压缩包，可以使用以下命令：

```bash
make package
```

这将生成一个名为 `runner-{version}-{os}-{arch}.tar.gz` 的压缩包，其中包含了完整的目录结构和所有必要的文件。