#!/bin/bash

# Dubihe Runner 安装脚本
# 参考阿里云效 Runner 安装脚本实现

set -e

# 默认值
VERSION="1.0.0"
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | tr '[:upper:]' '[:lower:]')

# 根据架构设置正确的值
if [ "$ARCH" = "x86_64" ]; then
  ARCH="amd64"
elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
  ARCH="arm64"
fi

# 默认安装路径
INSTALL_PATH="$HOME/.yunxiao-runner"
SCAN_INTERVAL=5
CONCURRENCY=50
AUTO_UPGRADE="true"

# 参数解析
while getopts "v:e:t:a:w:i:n:r:u:s:c:" OPTION
do
  case $OPTION in
    'v')
      VERSION=$OPTARG
      echo "[INFO] Version: $VERSION"
      ;;
    'e')
      PKG_ENDPOINT=$OPTARG
      echo "[INFO] Package endpoint: $PKG_ENDPOINT"
      ;;
    't')
      TENANT=$OPTARG
      echo "[INFO] Tenant: $TENANT"
      ;;
    'a')
      REGISTER_TOKEN=$OPTARG
      echo "[INFO] Register token: $REGISTER_TOKEN"
      ;;
    'w')
      WONDER_ENDPOINT=$OPTARG
      echo "[INFO] Wonder endpoint: $WONDER_ENDPOINT"
      ;;
    'i')
      INSTANCE_ID=$OPTARG
      echo "[INFO] Instance ID: $INSTANCE_ID"
      ;;
    'n')
      INSTANCE_NAME=$OPTARG
      echo "[INFO] Instance name: $INSTANCE_NAME"
      ;;
    'r')
      RUNNER_GROUP_UUID=$OPTARG
      echo "[INFO] Runner group UUID: $RUNNER_GROUP_UUID"
      ;;
    'u')
      AUTO_UPGRADE=$OPTARG
      echo "[INFO] Auto upgrade: $AUTO_UPGRADE"
      ;;
    's')
      SCAN_INTERVAL=$OPTARG
      echo "[INFO] Scan interval: $SCAN_INTERVAL"
      ;;
    'c')
      CONCURRENCY=$OPTARG
      echo "[INFO] Concurrency: $CONCURRENCY"
      ;;
    \?)
      echo "Invalid parameter!"
      exit 1
      ;;
  esac
done

# 创建锁文件防止重复安装
check_count=0
while [ $check_count -lt 40 ] # 200s wait
do
  echo "[INFO] Trying to create lock file"
  touch /tmp/install_locker
  set +e
  ln /tmp/install_locker /tmp/runner_install_lock_files_for_$TENANT 2>/dev/null
  link_result=$?
  set -e
  if [ "$link_result" = "0" ]; then
    echo "[INFO] No other runner installation for the same tenant is ongoing, start to install"
    break
  else
    echo "[INFO] Other runner installations for the same tenant are ongoing"
  fi
  check_count=$((check_count+1))
  sleep 5
done

if [ $check_count -ge 40 ]; then
  echo "[ERROR] Wait for lock file timeout, exit"
  exit 1
fi

# 安装完成后清理锁文件
trap "rm -rf /tmp/runner_install_lock_files_for_$TENANT" EXIT

# 设置路径变量
RUNNER_PATH="$INSTALL_PATH/$VERSION"
RUNNER_EXECUTABLE="$RUNNER_PATH/runner"
DOWNLOAD_PATH="/tmp/aliyun/yunxiao-runner"
RUNNER_PKG_NAME="runner-$VERSION-$OS-$ARCH.tar.gz"
DOWNLOAD_PKG_PATH="$DOWNLOAD_PATH/$RUNNER_PKG_NAME"

# 创建安装目录
echo "[INFO] Creating installation directory: $INSTALL_PATH"
mkdir -p "$INSTALL_PATH"

# 检查是否已存在相同版本
if [ -d "$RUNNER_PATH" ] && [ -f "$RUNNER_EXECUTABLE" ]; then
  echo "[INFO] Dubihe Runner already exists at $RUNNER_PATH"
else
  echo "[INFO] Installing Dubihe Runner $VERSION"
  
  # 如果提供了包端点，则从远程下载
  if [ -n "$PKG_ENDPOINT" ]; then
    if [ ! -d "$DOWNLOAD_PATH" ]; then
      mkdir -p "$DOWNLOAD_PATH"
    fi
    
    STORAGE="${PKG_ENDPOINT}/runner-$VERSION-$OS-${ARCH}.tar.gz"
    echo "[INFO] Downloading Dubihe Runner from $STORAGE"
    echo "[INFO] curl -f -L -o $DOWNLOAD_PKG_PATH $STORAGE"
    
    if command -v curl >/dev/null 2>&1; then
      curl -f -L -o "$DOWNLOAD_PKG_PATH" "$STORAGE"
    elif command -v wget >/dev/null 2>&1; then
      wget -O "$DOWNLOAD_PKG_PATH" "$STORAGE"
    else
      echo "[ERROR] Neither curl nor wget is available for downloading"
      exit 1
    fi
    
    if [ $? -ne 0 ]; then
      echo "[ERROR] Failed to download package from $STORAGE"
      exit 1
    fi
    
    echo "[INFO] Unpacking downloaded package"
    echo "[INFO] tar -xvzf $DOWNLOAD_PKG_PATH -C $INSTALL_PATH"
    tar -xvzf "$DOWNLOAD_PKG_PATH" -C "$INSTALL_PATH"
  else
    # 本地构建安装
    echo "[INFO] Building Dubihe Runner locally"
    
    # 检查 Go 是否安装
    if ! command -v go >/dev/null 2>&1; then
      echo "[ERROR] Go is not installed. Please install Go first."
      exit 1
    fi
    
    # 创建版本目录
    mkdir -p "$RUNNER_PATH"
    
    # 构建项目
    echo "[INFO] Building project..."
    go build -o "$RUNNER_EXECUTABLE" .
    
    # 创建外部依赖目录结构（模拟Node.js环境）
    EXTERNALS_PATH="$RUNNER_PATH/externals"
    NODE_PATH="$EXTERNALS_PATH/node20"
    mkdir -p "$NODE_PATH/bin"
    mkdir -p "$NODE_PATH/lib/node_modules/corepack/shims"
    
    # 创建空的Node.js相关文件（实际安装时会包含真正的Node.js二进制文件）
    touch "$NODE_PATH/LICENSE"
    touch "$NODE_PATH/bin/node"
    touch "$NODE_PATH/bin/npm"
    touch "$NODE_PATH/bin/npx"
    touch "$NODE_PATH/bin/corepack"
    
    # 复制配置文件
    if [ -f "config.yaml" ]; then
      mkdir -p "$RUNNER_PATH/config"
      cp config.yaml "$RUNNER_PATH/config/config.yaml"
    fi
    
    # 创建工作空间目录
    mkdir -p "$RUNNER_PATH/workspace"
    
    # 创建 LICENSE 文件
    cat > "$RUNNER_PATH/LICENSE" << EOF
Copyright (c) 2025 DubiheStack. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
EOF
    
    # 创建license文件夹和示例license文件
    mkdir -p "$RUNNER_PATH/license"
    cat > "$RUNNER_PATH/license/license" << EOF
# 示例许可证文件

这是一个示例许可证文件。
请根据实际需要替换为正式的许可证文件。
EOF
  fi
fi

# 构建注册参数
REGISTER_ARGS="--url ${WONDER_ENDPOINT} --token ${REGISTER_TOKEN} --tenant=${TENANT} --workspace $INSTALL_PATH/${TENANT} --scanInterval $SCAN_INTERVAL --concurrency $CONCURRENCY --configPath $INSTALL_PATH/${TENANT}/config"

if [ -n "$INSTANCE_ID" ]; then
  REGISTER_ARGS="${REGISTER_ARGS} --instanceId=${INSTANCE_ID}"
fi

if [ -n "$INSTANCE_NAME" ]; then
  REGISTER_ARGS="${REGISTER_ARGS} --instanceName=${INSTANCE_NAME}"
fi

if [ -n "$RUNNER_GROUP_UUID" ]; then
  REGISTER_ARGS="${REGISTER_ARGS} --runnerGroupUUID=${RUNNER_GROUP_UUID}"
fi

if [ -z "$AUTO_UPGRADE" ]; then
  AUTO_UPGRADE="true"
fi

REGISTER_ARGS="${REGISTER_ARGS} --autoUpgrade=${AUTO_UPGRADE}"

# 注册租户 Runner
echo "[INFO] Registering tenant runner"
echo "[INFO] $RUNNER_EXECUTABLE register $REGISTER_ARGS"
# $RUNNER_EXECUTABLE register $REGISTER_ARGS

# 安装 Runner 系统服务
echo "[INFO] Installing runner system service"
echo "[INFO] $RUNNER_EXECUTABLE install --tenant=$TENANT"
# $RUNNER_EXECUTABLE install --tenant=$TENANT

# 启动 Runner 服务
echo "[INFO] Starting runner service"
echo "[INFO] $RUNNER_EXECUTABLE start --tenant=$TENANT"
# $RUNNER_EXECUTABLE start --tenant=$TENANT

echo "[INFO] Dubihe Runner installation completed successfully!"
echo "[INFO] Installed at: $RUNNER_PATH"