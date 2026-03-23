package war

import (
	"context"

	buildimage "github.com/DubiheStack/dubihe-runner/engine/buildimage"
	"github.com/DubiheStack/dubihe-runner/resource"
)

// War WAR构建器
type War struct{}

// Ensure War implements buildimage.Builder interface
var _ buildimage.Builder = (*War)(nil)

// Builder 是公开的WAR构建器实例
var Builder = &War{}

// init 初始化时注册构建器
func init() {
	buildimage.RegisterBuilder("java:war", func() buildimage.Builder {
		return &War{}
	})
}

// Prepare 准备构建环境
func (war *War) Prepare(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	return buildimage.CreateDockerfile(config, workDir, "war")
}

// Build 执行构建
func (war *War) Build(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	return buildimage.BuildDockerImage(ctx, config, workDir)
}

// Push 推送构建产物
func (war *War) Push(ctx context.Context, config *resource.BuildImageStep) error {
	return buildimage.PushImage(ctx, config)
}

// SendResult 发送构建结果
func (war *War) SendResult(ctx context.Context, config *resource.BuildImageStep) error {
	return buildimage.SendBuildResult(ctx, config)
}
