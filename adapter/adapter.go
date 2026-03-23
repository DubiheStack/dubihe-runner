package adapter

import (
	"context"
	"github.com/DubiheStack/dubihe-runner/engine"
	artifactuploadoss_engine "github.com/DubiheStack/dubihe-runner/engine/artifactuploadoss"
	buildimage_engine "github.com/DubiheStack/dubihe-runner/engine/buildimage"
	// 导入所有构建器包，确保它们的 init() 函数能够执行注册
	_ "github.com/DubiheStack/dubihe-runner/engine/buildimage/golang"
	_ "github.com/DubiheStack/dubihe-runner/engine/buildimage/java/gradle"
	_ "github.com/DubiheStack/dubihe-runner/engine/buildimage/java/jar"
	_ "github.com/DubiheStack/dubihe-runner/engine/buildimage/java/maven"
	_ "github.com/DubiheStack/dubihe-runner/engine/buildimage/java/war"
	_ "github.com/DubiheStack/dubihe-runner/engine/buildimage/node"
	_ "github.com/DubiheStack/dubihe-runner/engine/buildimage/php"
	_ "github.com/DubiheStack/dubihe-runner/engine/buildimage/python"
	_ "github.com/DubiheStack/dubihe-runner/engine/buildimage/scala/sbt"
	oss_engine "github.com/DubiheStack/dubihe-runner/engine/oss"
	"github.com/DubiheStack/dubihe-runner/resource"
	"github.com/sirupsen/logrus"
)

// EngineAdapter 适配器，用于增强引擎功能
type EngineAdapter struct {
	engine engine.Engine
	logger *logrus.Entry
}

// NewEngineAdapter 创建新的引擎适配器
func NewEngineAdapter(engine engine.Engine, logger *logrus.Entry) *EngineAdapter {
	if logger == nil {
		logger = logrus.WithField("component", "engine-adapter")
	}
	return &EngineAdapter{
		engine: engine,
		logger: logger,
	}
}

// Name 返回引擎名称
func (a *EngineAdapter) Name() string {
	return a.engine.Name()
}

// Setup 准备执行环境
func (a *EngineAdapter) Setup(ctx context.Context, job *resource.Job, opts *engine.Options) error {
	return a.engine.Setup(ctx, job, opts)
}

// Execute 执行步骤，拦截特殊步骤类型
func (a *EngineAdapter) Execute(ctx context.Context, step *resource.Step, opts *engine.Options) (*engine.StepResult, error) {
	switch step.Step {
	case resource.StepTypeOSSUpload:
		a.logger.WithField("step", step.Key).Info("Handling OSSUpload step with specialized processor")
		// 创建OSS上传器并执行上传
		uploader := oss_engine.New(a.logger)
		return uploader.Upload(ctx, step, opts)
	case resource.StepTypeBuildImage:
		a.logger.WithField("step", step.Key).Info("Handling buildImage step with specialized processor")
		// 创建制作镜像引擎并执行
		buildImageEngine := buildimage_engine.New()
		return buildImageEngine.Execute(ctx, step, opts)
	case resource.StepTypeArtifactUploadOss:
		a.logger.WithField("step", step.Key).Info("Handling artifactUploadOss step with specialized processor")
		// 创建构件上传OSS引擎并执行
		artifactUploadOssEngine := artifactuploadoss_engine.New(a.logger)
		return artifactUploadOssEngine.Execute(ctx, step, opts)
	}

	// 对于其他步骤，使用原始引擎
	return a.engine.Execute(ctx, step, opts)
}

// Destroy 清理执行环境
func (a *EngineAdapter) Destroy(ctx context.Context) error {
	return a.engine.Destroy(ctx)
}

// Ping 检查引擎状态
func (a *EngineAdapter) Ping(ctx context.Context) error {
	return a.engine.Ping(ctx)
}
