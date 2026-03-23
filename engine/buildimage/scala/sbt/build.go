package sbt

import (
	"context"

	buildimage "github.com/DubiheStack/dubihe-runner/engine/buildimage"
	"github.com/DubiheStack/dubihe-runner/resource"
)

// Sbt SBT构建器
type Sbt struct{}

// Ensure Sbt implements buildimage.Builder interface
var _ buildimage.Builder = (*Sbt)(nil)

// Builder 是公开的SBT构建器实例
var Builder = &Sbt{}

// init 初始化时注册构建器
func init() {
	buildimage.RegisterBuilder("scala:sbt", func() buildimage.Builder {
		return &Sbt{}
	})
}

// Prepare 准备构建环境
func (sbt *Sbt) Prepare(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	return buildimage.CreateDockerfile(config, workDir, "scala")
}

// Build 执行构建
func (sbt *Sbt) Build(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	return buildimage.BuildDockerImage(ctx, config, workDir)
}

// Push 推送构建产物
func (sbt *Sbt) Push(ctx context.Context, config *resource.BuildImageStep) error {
	return buildimage.PushImage(ctx, config)
}

// SendResult 发送构建结果
func (sbt *Sbt) SendResult(ctx context.Context, config *resource.BuildImageStep) error {
	return buildimage.SendBuildResult(ctx, config)
}
