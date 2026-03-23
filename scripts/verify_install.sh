#!/bin/bash

# Dubihe Runner 安装验证脚本

set -e

# 默认安装路径
INSTALL_PATH="$HOME/.yunxiao-runner"
VERSION="1.0.0"

# 检查安装路径是否存在
if [ ! -d "$INSTALL_PATH" ]; then
    echo "[ERROR] Installation path $INSTALL_PATH does not exist"
    exit 1
fi

echo "[INFO] Found installation path: $INSTALL_PATH"

# 检查版本目录是否存在
RUNNER_PATH="$INSTALL_PATH/$VERSION"
if [ ! -d "$RUNNER_PATH" ]; then
    echo "[ERROR] Version directory $RUNNER_PATH does not exist"
    exit 1
fi

echo "[INFO] Found version directory: $RUNNER_PATH"

# 检查关键文件是否存在
REQUIRED_FILES=(
    "$RUNNER_PATH/runner"
    "$RUNNER_PATH/config/config.yaml"
    "$RUNNER_PATH/LICENSE"
    "$RUNNER_PATH/license/license"
    "$RUNNER_PATH/externals/node20/LICENSE"
    "$RUNNER_PATH/externals/node20/bin/node"
    "$RUNNER_PATH/externals/node20/bin/npm"
    "$RUNNER_PATH/externals/node20/bin/npx"
    "$RUNNER_PATH/externals/node20/bin/corepack"
)

for file in "${REQUIRED_FILES[@]}"; do
    if [ ! -f "$file" ]; then
        echo "[ERROR] Required file $file does not exist"
        exit 1
    fi
    echo "[INFO] Found required file: $file"
done

# 检查关键目录是否存在
REQUIRED_DIRS=(
    "$RUNNER_PATH/workspace"
    "$RUNNER_PATH/license"
    "$RUNNER_PATH/externals/node20/lib/node_modules/corepack/shims"
)

for dir in "${REQUIRED_DIRS[@]}"; do
    if [ ! -d "$dir" ]; then
        echo "[ERROR] Required directory $dir does not exist"
        exit 1
    fi
    echo "[INFO] Found required directory: $dir"
done

echo "[SUCCESS] All required files and directories are present!"
echo "[INFO] Installation verification completed successfully."

# 显示目录结构
echo ""
echo "Directory structure:"
find "$INSTALL_PATH" -type d | sed 's/[^/]*\//|-/g;s/|-/ |-/g'