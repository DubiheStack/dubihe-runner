package python

import (
	"context"

	buildimage "github.com/DubiheStack/dubihe-runner/engine/buildimage"
	"github.com/DubiheStack/dubihe-runner/resource"
)

// Python Python构建器
type Python struct{}

// Ensure Python implements buildimage.Builder interface
var _ buildimage.Builder = (*Python)(nil)

// Builder 是公开的Python构建器实例
var Builder = &Python{}

// init 初始化时注册构建器
func init() {
	buildimage.RegisterBuilder("python", func() buildimage.Builder {
		return &Python{}
	})
}

// Prepare 准备构建环境
func (python *Python) Prepare(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	return buildimage.CreateDockerfile(config, workDir, "python")
}

// Build 执行构建
func (python *Python) Build(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	return buildimage.BuildDockerImage(ctx, config, workDir)
}

// Push 推送构建产物
func (python *Python) Push(ctx context.Context, config *resource.BuildImageStep) error {
	return buildimage.PushImage(ctx, config)
}

// SendResult 发送构建结果
func (python *Python) SendResult(ctx context.Context, config *resource.BuildImageStep) error {
	return buildimage.SendBuildResult(ctx, config)
}
