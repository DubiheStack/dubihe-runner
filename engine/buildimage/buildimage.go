// Package buildimage 实现内部镜像构建引擎
package buildimage

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/DubiheStack/dubihe-runner/engine"
	"github.com/DubiheStack/dubihe-runner/resource"
	"github.com/sirupsen/logrus"
)

// Builder 构建器接口，定义了构建逻辑的通用方法
type Builder interface {
	// Prepare 准备构建环境
	Prepare(ctx context.Context, config *resource.BuildImageStep, workDir string) error
	// Build 执行构建
	Build(ctx context.Context, config *resource.BuildImageStep, workDir string) error
	// Push 推送构建产物
	Push(ctx context.Context, config *resource.BuildImageStep) error
	// SendResult 发送构建结果
	SendResult(ctx context.Context, config *resource.BuildImageStep) error
}

// Engine 内部镜像构建引擎
type Engine struct {
	log *logrus.Entry
}

// New 创建内部镜像构建引擎
func New() *Engine {
	return &Engine{
		log: logrus.WithField("engine", "buildimage"),
	}
}

// Name 返回引擎名称
func (e *Engine) Name() string {
	return "buildimage"
}

// Ping 检查引擎状态
func (e *Engine) Ping(ctx context.Context) error {
	// 检查是否安装了docker
	if err := exec.CommandContext(ctx, "docker", "--version").Run(); err != nil {
		return fmt.Errorf("docker is not available: %w", err)
	}
	return nil
}

// Setup 准备执行环境
func (e *Engine) Setup(ctx context.Context, job *resource.Job, opts *engine.Options) error {
	e.log.WithField("job", job.Key).Info("Setting up build image environment")
	return nil
}

// Execute 执行内部镜像构建步骤
func (e *Engine) Execute(ctx context.Context, step *resource.Step, opts *engine.Options) (*engine.StepResult, error) {
	e.log.WithField("step", step.Key).WithField("name", step.Name).Info("Executing build image step")

	startTime := time.Now()
	result := &engine.StepResult{}

	// 检查步骤类型是否为制作镜像
	if step.Step != resource.StepTypeBuildImage {
		return result, fmt.Errorf("step type is not buildImage: %s", step.Step)
	}

	// 获取制作镜像步骤参数
	buildImageStep, ok := step.With.(*resource.BuildImageStep)
	if !ok {
		return result, fmt.Errorf("invalid buildImage step configuration")
	}

	// 根据配置加载对应的构建器
	builder, err := e.loadBuilder(buildImageStep)
	if err != nil {
		return result, fmt.Errorf("failed to load builder: %w", err)
	}

	// 执行构建逻辑
	if err := e.executeBuild(ctx, builder, buildImageStep, opts.WorkDir); err != nil {
		result.ExitCode = 1
		if !step.ContinueOnError {
			return result, fmt.Errorf("build image step %s failed: %w", step.Key, err)
		}
		e.log.WithField("step", step.Key).WithField("exitCode", result.ExitCode).Warn("Build image step failed but continuing")
	} else {
		e.log.WithField("step", step.Key).Info("Build image step completed successfully")
	}

	result.Duration = time.Since(startTime).Milliseconds()
	return result, nil
}

// BuilderFactory 构建器工厂，用于创建不同类型的构建器
var BuilderFactory = make(map[string]func() Builder)

// RegisterBuilder 注册构建器
func RegisterBuilder(key string, factory func() Builder) {
	BuilderFactory[key] = factory
}

// loadBuilder 根据配置加载对应的构建器
func (e *Engine) loadBuilder(config *resource.BuildImageStep) (Builder, error) {
	language := config.Language
	switch language {
	case "java":
		switch config.Tool {
		case "maven":
			if factory, exists := BuilderFactory["java:maven"]; exists {
				return factory(), nil
			}
			return nil, fmt.Errorf("maven builder not registered")
		case "gradle":
			if factory, exists := BuilderFactory["java:gradle"]; exists {
				return factory(), nil
			}
			return nil, fmt.Errorf("gradle builder not registered")
		case "jar":
			if factory, exists := BuilderFactory["java:jar"]; exists {
				return factory(), nil
			}
			return nil, fmt.Errorf("jar builder not registered")
		case "war":
			if factory, exists := BuilderFactory["java:war"]; exists {
				return factory(), nil
			}
			return nil, fmt.Errorf("war builder not registered")
		}
		return nil, fmt.Errorf("can not find maven/gradle/jar/war config")
	case "python":
		if factory, exists := BuilderFactory["python"]; exists {
			return factory(), nil
		}
		return nil, fmt.Errorf("python builder not registered")
	case "php":
		if factory, exists := BuilderFactory["php"]; exists {
			return factory(), nil
		}
		return nil, fmt.Errorf("php builder not registered")
	case "node":
		if factory, exists := BuilderFactory["node"]; exists {
			return factory(), nil
		}
		return nil, fmt.Errorf("node builder not registered")
	case "golang":
		if factory, exists := BuilderFactory["golang"]; exists {
			return factory(), nil
		}
		return nil, fmt.Errorf("golang builder not registered")
	case "scala":
		if factory, exists := BuilderFactory["scala:sbt"]; exists {
			return factory(), nil
		}
		return nil, fmt.Errorf("scala builder not registered")
	default:
		return nil, fmt.Errorf("can not handle language %s", language)
	}
}

// executeBuild 执行构建流程
func (e *Engine) executeBuild(ctx context.Context, builder Builder, config *resource.BuildImageStep, workDir string) error {
	e.log.Info("Starting internal build image process")

	// 1. 准备构建环境
	if err := builder.Prepare(ctx, config, workDir); err != nil {
		return fmt.Errorf("failed to prepare build environment: %w", err)
	}

	// 2. 执行构建
	if err := builder.Build(ctx, config, workDir); err != nil {
		return fmt.Errorf("failed to build: %w", err)
	}

	// 3. 推送产物
	if err := builder.Push(ctx, config); err != nil {
		return fmt.Errorf("failed to push: %w", err)
	}

	// 4. 发送构建结果
	if err := builder.SendResult(ctx, config); err != nil {
		return fmt.Errorf("failed to send build result: %w", err)
	}

	e.log.Info("Internal build image process completed successfully")
	return nil
}

// Destroy 清理执行环境
func (e *Engine) Destroy(ctx context.Context) error {
	e.log.Info("Destroying build image environment")
	return nil
}
