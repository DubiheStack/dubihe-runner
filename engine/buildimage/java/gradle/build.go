package gradle

import (
	"context"

	buildimage "github.com/DubiheStack/dubihe-runner/engine/buildimage"
	"github.com/DubiheStack/dubihe-runner/resource"
)

// Gradle Gradle构建器
type Gradle struct{}

// Ensure Gradle implements buildimage.Builder interface
var _ buildimage.Builder = (*Gradle)(nil)

// Builder 是公开的Gradle构建器实例
var Builder = &Gradle{}

// init 初始化时注册构建器
func init() {
	buildimage.RegisterBuilder("java:gradle", func() buildimage.Builder {
		return &Gradle{}
	})
}

// Prepare 准备构建环境
func (gradle *Gradle) Prepare(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	return buildimage.CreateDockerfile(config, workDir, "gradle")
}

// Build 执行构建
func (gradle *Gradle) Build(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	return buildimage.BuildDockerImage(ctx, config, workDir)
}

// Push 推送构建产物
func (gradle *Gradle) Push(ctx context.Context, config *resource.BuildImageStep) error {
	return buildimage.PushImage(ctx, config)
}

// SendResult 发送构建结果
func (gradle *Gradle) SendResult(ctx context.Context, config *resource.BuildImageStep) error {
	return buildimage.SendBuildResult(ctx, config)
}
