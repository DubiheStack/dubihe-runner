package maven

import (
	"context"

	buildimage "github.com/DubiheStack/dubihe-runner/engine/buildimage"
	"github.com/DubiheStack/dubihe-runner/resource"
)

// Maven Maven构建器
type Maven struct{}

// Ensure Maven implements buildimage.Builder interface
var _ buildimage.Builder = (*Maven)(nil)

// Builder 是公开的Maven构建器实例
var Builder = &Maven{}

// init 初始化时注册构建器
func init() {
	buildimage.RegisterBuilder("java:maven", func() buildimage.Builder {
		return &Maven{}
	})
}

// Prepare 准备构建环境
func (maven *Maven) Prepare(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	return buildimage.CreateDockerfile(config, workDir, "maven")
}

// Build 执行构建
func (maven *Maven) Build(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	return buildimage.BuildDockerImage(ctx, config, workDir)
}

// Push 推送构建产物
func (maven *Maven) Push(ctx context.Context, config *resource.BuildImageStep) error {
	return buildimage.PushImage(ctx, config)
}

// SendResult 发送构建结果
func (maven *Maven) SendResult(ctx context.Context, config *resource.BuildImageStep) error {
	return buildimage.SendBuildResult(ctx, config)
}

// toJibArgs 生成JIB参数（参考原bake实现）
func toJibArgs(config *resource.BuildImageStep) ([]string, error) {
	// 实现类似原bake的功能
	args := []string{
		"com.google.cloud.tools:jib-maven-plugin:1.1.1:build",
		"-Dimage=" + config.PackageVersion,
	}

	if config.Jar != "" {
		args = append(args, "-f="+config.Jar)
	}

	return args, nil
}
