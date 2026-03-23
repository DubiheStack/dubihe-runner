package golang

import (
	"context"

	buildimage "github.com/DubiheStack/dubihe-runner/engine/buildimage"
	"github.com/DubiheStack/dubihe-runner/resource"
)

// Golang Go构建器
type Golang struct{}

// Ensure Golang implements buildimage.Builder interface
var _ buildimage.Builder = (*Golang)(nil)

// Builder 是公开的Go构建器实例
var Builder = &Golang{}

// init 初始化时注册构建器
func init() {
	buildimage.RegisterBuilder("golang", func() buildimage.Builder {
		return &Golang{}
	})
}

// Prepare 准备构建环境
func (golang *Golang) Prepare(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	return buildimage.CreateDockerfile(config, workDir, "golang")
}

// Build 执行构建
func (golang *Golang) Build(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	return buildimage.BuildDockerImage(ctx, config, workDir)
}

// Push 推送构建产物
func (golang *Golang) Push(ctx context.Context, config *resource.BuildImageStep) error {
	return buildimage.PushImage(ctx, config)
}

// SendResult 发送构建结果
func (golang *Golang) SendResult(ctx context.Context, config *resource.BuildImageStep) error {
	return buildimage.SendBuildResult(ctx, config)
}
