# Dubihe Runner Makefile

# 版本号
VERSION ?= 1.0.0

# 架构和操作系统
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# 安装路径
INSTALL_PATH ?= ~/.dubihe-runner
RUNNER_PATH = $(INSTALL_PATH)/$(VERSION)

# 打包名称
PKG_NAME = runner-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz

# Node.js 版本和路径
NODE_VERSION = 20
EXTERNALS_PATH = $(RUNNER_PATH)/externals
NODE_PATH = $(EXTERNALS_PATH)/node$(NODE_VERSION)

# 默认目标
.PHONY: all
all: build

# 构建二进制文件
.PHONY: build
build:
	go build -o dubihe-runner .

# 安装Runner到指定目录
install: build
	@echo "Installing Dubihe Runner to $(RUNNER_PATH)..."
	# 创建必要的目录结构
	mkdir -p $(RUNNER_PATH)/config
	mkdir -p $(RUNNER_PATH)/workspace
	mkdir -p $(RUNNER_PATH)/externals/node20
	# 复制Runner可执行文件
	cp dubihe-runner $(RUNNER_PATH)/
	# 复制配置文件模板
	cp config/config.yaml $(RUNNER_PATH)/config/
	# 创建license文件夹和示例license文件
	$(MAKE) install-license
	@echo "Installation completed!"

# 创建license文件夹和示例license文件
install-license:
	mkdir -p $(RUNNER_PATH)/license
	@echo "# 示例许可证文件" > $(RUNNER_PATH)/license/license
	@echo "" >> $(RUNNER_PATH)/license/license
	@echo "这是一个示例许可证文件。" >> $(RUNNER_PATH)/license/license
	@echo "请根据实际需要替换为正式的许可证文件。" >> $(RUNNER_PATH)/license/license
	@echo "" >> $(RUNNER_PATH)/license/license
	@echo "# License Properties" >> $(RUNNER_PATH)/license/license
	@echo "productName=Dubihe Runner" >> $(RUNNER_PATH)/license/license
	@echo "version=1.0.0" >> $(RUNNER_PATH)/license/license
	@echo "expireDate=2030-12-31" >> $(RUNNER_PATH)/license/license
	@echo "# 在生产环境中，请填写实际的服务器信息以启用验证" >> $(RUNNER_PATH)/license/license
	@echo "cpuInfo=" >> $(RUNNER_PATH)/license/license
	@echo "diskInfo=" >> $(RUNNER_PATH)/license/license
	@echo "macAddress=" >> $(RUNNER_PATH)/license/license

# 打包发布版本
.PHONY: package
package: build
	@echo "Packaging Dubihe Runner $(VERSION)..."

	# 创建临时目录结构
	mkdir -p dist/$(VERSION)
	cp dubihe-runner dist/$(VERSION)/runner

	# 创建外部依赖目录结构
	mkdir -p dist/$(VERSION)/externals

	# 创建配置目录
	mkdir -p dist/$(VERSION)/config
	cp config.yaml dist/$(VERSION)/config/config.yaml

	# 创建工作空间目录
	mkdir -p dist/$(VERSION)/workspace

	# 创建license文件夹和示例license文件
	mkdir -p dist/$(VERSION)/license
	@echo "# 示例许可证文件" > dist/$(VERSION)/license/license
	@echo "" >> dist/$(VERSION)/license/license
	@echo "这是一个示例许可证文件。" >> dist/$(VERSION)/license/license
	@echo "请根据实际需要替换为正式的许可证文件。" >> dist/$(VERSION)/license/license

	# 打包
	cd dist && tar -czvf ../$(PKG_NAME) $(VERSION)

	# 清理临时目录
	rm -rf dist

	@echo "Package created: $(PKG_NAME)"

# 模拟阿里云效安装后的目录结构
.PHONY: simulate-install
simulate-install: build
	@echo "Simulating Dubihe Runner installation..."

	# 创建安装目录
	mkdir -p $(INSTALL_PATH)

	# 创建版本目录
	mkdir -p $(RUNNER_PATH)

	# 复制二进制文件
	cp dubihe-runner $(RUNNER_PATH)/runner

	# 创建外部依赖目录结构（模拟Node.js环境）
	mkdir -p $(NODE_PATH)/bin
	mkdir -p $(NODE_PATH)/lib/node_modules/corepack/shims

	# 创建空的Node.js相关文件（实际安装时会包含真正的Node.js二进制文件）
	touch $(NODE_PATH)/LICENSE
	touch $(NODE_PATH)/bin/node
	touch $(NODE_PATH)/bin/npm
	touch $(NODE_PATH)/bin/npx
	touch $(NODE_PATH)/bin/corepack

	# 创建配置目录
	mkdir -p $(RUNNER_PATH)/config
	cp config.yaml $(RUNNER_PATH)/config/config.yaml

	# 创建工作空间目录
	mkdir -p $(RUNNER_PATH)/workspace

	# 创建license文件夹和示例license文件
	mkdir -p $(RUNNER_PATH)/license
	@echo "# 示例许可证文件" > $(RUNNER_PATH)/license/license
	@echo "" >> $(RUNNER_PATH)/license/license
	@echo "这是一个示例许可证文件。" >> $(RUNNER_PATH)/license/license
	@echo "请根据实际需要替换为正式的许可证文件。" >> $(RUNNER_PATH)/license/license

	@echo "Dubihe Runner directory structure simulated at $(INSTALL_PATH)"
	@echo "Directory structure:"
	@find $(INSTALL_PATH) -type d | sed 's/[^/]*\//|-/g;s/|-/ |-/g'

# 清理构建产物
.PHONY: clean
clean:
	rm -f dubihe-runner
	rm -f $(PKG_NAME)
	rm -rf dist

# 运行测试
.PHONY: test
test:
	go test ./...

# 格式化代码
.PHONY: fmt
fmt:
	go fmt ./...

# 检查代码
.PHONY: vet
vet:
	go vet ./...