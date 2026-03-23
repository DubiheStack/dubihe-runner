// Package engine 定义执行引擎接口
package engine

import (
	"context"
	"io"

	"github.com/DubiheStack/dubihe-runner/resource"
)

// Engine 执行引擎接口
type Engine interface {
	// Name 返回引擎名称
	Name() string

	// Setup 准备执行环境
	Setup(ctx context.Context, job *resource.Job, opts *Options) error

	// Execute 执行步骤
	Execute(ctx context.Context, step *resource.Step, opts *Options) (*StepResult, error)

	// Destroy 清理执行环境
	Destroy(ctx context.Context) error

	// Ping 检查引擎状态
	Ping(ctx context.Context) error
}

// Options 执行选项
type Options struct {
	// WorkDir 工作目录
	WorkDir string
	// Env 环境变量
	Env map[string]string
	// Stdout 标准输出
	Stdout io.Writer
	// Stderr 标准错误
	Stderr io.Writer
	// Timeout 超时时间(秒)
	Timeout int
}

// StepResult 步骤执行结果
type StepResult struct {
	// ExitCode 退出码
	ExitCode int
	// Stdout 标准输出内容
	Stdout string
	// Stderr 标准错误内容
	Stderr string
	// Duration 执行时长(毫秒)
	Duration int64
}

// NewOptions 创建默认选项
func NewOptions() *Options {
	return &Options{
		Env:     make(map[string]string),
		Timeout: 3600, // 默认1小时
	}
}

// WithEnv 添加环境变量
func (o *Options) WithEnv(key, value string) *Options {
	if o.Env == nil {
		o.Env = make(map[string]string)
	}
	o.Env[key] = value
	return o
}

// WithEnvMap 批量添加环境变量
func (o *Options) WithEnvMap(env map[string]string) *Options {
	if o.Env == nil {
		o.Env = make(map[string]string)
	}
	for k, v := range env {
		o.Env[k] = v
	}
	return o
}

// MergeEnv 合并环境变量
func MergeEnv(envs ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, env := range envs {
		for k, v := range env {
			result[k] = v
		}
	}
	return result
}
