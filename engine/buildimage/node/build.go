package node

import (
	"context"

	buildimage "github.com/DubiheStack/dubihe-runner/engine/buildimage"
	"github.com/DubiheStack/dubihe-runner/resource"
)

// Node Node.js构建器
type Node struct{}

// Ensure Node implements buildimage.Builder interface
var _ buildimage.Builder = (*Node)(nil)

// Builder 是公开的Node.js构建器实例
var Builder = &Node{}

// init 初始化时注册构建器
func init() {
	buildimage.RegisterBuilder("node", func() buildimage.Builder {
		return &Node{}
	})
}

// Prepare 准备构建环境
func (node *Node) Prepare(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	return buildimage.CreateDockerfile(config, workDir, "node")
}

// Build 执行构建
func (node *Node) Build(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	return buildimage.BuildDockerImage(ctx, config, workDir)
}

// Push 推送构建产物
func (node *Node) Push(ctx context.Context, config *resource.BuildImageStep) error {
	return buildimage.PushImage(ctx, config)
}

// SendResult 发送构建结果
func (node *Node) SendResult(ctx context.Context, config *resource.BuildImageStep) error {
	return buildimage.SendBuildResult(ctx, config)
}
