package php

import (
	"context"

	buildimage "github.com/DubiheStack/dubihe-runner/engine/buildimage"
	"github.com/DubiheStack/dubihe-runner/resource"
)

// Php PHP构建器
type Php struct{}

// Ensure Php implements buildimage.Builder interface
var _ buildimage.Builder = (*Php)(nil)

// Builder 是公开的PHP构建器实例
var Builder = &Php{}

// init 初始化时注册构建器
func init() {
	buildimage.RegisterBuilder("php", func() buildimage.Builder {
		return &Php{}
	})
}

// Prepare 准备构建环境
func (php *Php) Prepare(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	return buildimage.CreateDockerfile(config, workDir, "php")
}

// Build 执行构建
func (php *Php) Build(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	return buildimage.BuildDockerImage(ctx, config, workDir)
}

// Push 推送构建产物
func (php *Php) Push(ctx context.Context, config *resource.BuildImageStep) error {
	return buildimage.PushImage(ctx, config)
}

// SendResult 发送构建结果
func (php *Php) SendResult(ctx context.Context, config *resource.BuildImageStep) error {
	return buildimage.SendBuildResult(ctx, config)
}
