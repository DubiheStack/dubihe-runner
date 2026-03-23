// Package runner 实现流水线执行器
package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/DubiheStack/dubihe-runner/adapter"
	"github.com/DubiheStack/dubihe-runner/client"
	"github.com/DubiheStack/dubihe-runner/engine"
	"github.com/DubiheStack/dubihe-runner/engine/docker"
	"github.com/DubiheStack/dubihe-runner/engine/exec"
	"github.com/DubiheStack/dubihe-runner/resource"
	"github.com/DubiheStack/dubihe-runner/scm"
	"github.com/sirupsen/logrus"
)

// RunnerType Runner类型
type RunnerType string

const (
	// RunnerTypeDocker Docker执行器
	RunnerTypeDocker RunnerType = "docker"
	// RunnerTypeExe 本地执行器
	RunnerTypeExe RunnerType = "exe"
	// RunnerTypeExeDocker 混合执行器(默认)
	RunnerTypeExeDocker RunnerType = "exe_docker"
)

// ExecutionResult 流水线执行结果
type ExecutionResult struct {
	// Success 是否成功
	Success bool
	// Duration 执行时长(毫秒)
	Duration int64
	// Error 错误信息
	Error error
	// StageResults 阶段执行结果
	StageResults []*StageResult
}

// StageResult 阶段执行结果
type StageResult struct {
	// Key 阶段Key
	Key string
	// Name 阶段名称
	Name string
	// Success 是否成功
	Success bool
	// Duration 执行时长(毫秒)
	Duration int64
	// Error 错误信息
	Error error
	// JobResults 任务执行结果
	JobResults []*JobResult
}

// JobResult 任务执行结果
type JobResult struct {
	// Key 任务Key
	Key string
	// Name 任务名称
	Name string
	// Success 是否成功
	Success bool
	// Duration 执行时长(毫秒)
	Duration int64
	// Error 错误信息
	Error error
	// StepResults 步骤执行结果
	StepResults []*StepResult
}

// StepResult 步骤执行结果
type StepResult struct {
	// Key 步骤Key
	Key string
	// Name 步骤名称
	Name string
	// Success 是否成功
	Success bool
	// Duration 执行时长(毫秒)
	Duration int64
	// ExitCode 退出码
	ExitCode int
	// Output 输出内容
	Output string
}

// Runner 流水线执行器
type Runner struct {
	// Type 执行器类型
	Type RunnerType
	// WorkDir 工作目录
	WorkDir string
	// Env 环境变量
	Env map[string]string
	// Client API客户端
	Client *client.Client
	// Logger 日志记录器
	Logger *logrus.Entry
	// Stdout 标准输出
	Stdout io.Writer
	// Stderr 标准错误输出
	Stderr io.Writer

	// OnCloneStart 克隆开始回调
	OnCloneStart func(source string)
	// OnCloneComplete 克隆完成回调
	OnCloneComplete func(source string, err error)
	// OnJobStart 任务开始回调
	OnJobStart func(stage, job string)
	// OnJobComplete 任务完成回调
	OnJobComplete func(stage, job string, err error)
	// OnStepStart 步骤开始回调
	OnStepStart func(stage, job, step string)
	// OnStepComplete 步骤完成回调
	OnStepComplete func(stage, job, step string, result *engine.StepResult)
}

// Option Runner选项
type Option func(*Runner)

// WithType 设置执行器类型
func WithType(t RunnerType) Option {
	return func(r *Runner) {
		r.Type = t
	}
}

// WithWorkDir 设置工作目录
func WithWorkDir(dir string) Option {
	return func(r *Runner) {
		r.WorkDir = dir
	}
}

// WithEnv 设置环境变量
func WithEnv(env map[string]string) Option {
	return func(r *Runner) {
		r.Env = env
	}
}

// WithClient 设置API客户端
func WithClient(c *client.Client) Option {
	return func(r *Runner) {
		r.Client = c
	}
}

// WithLogger 设置日志记录器
func WithLogger(logger *logrus.Entry) Option {
	return func(r *Runner) {
		r.Logger = logger
	}
}

// WithStdout 设置标准输出
func WithStdout(w io.Writer) Option {
	return func(r *Runner) {
		r.Stdout = w
	}
}

// WithStderr 设置标准错误输出
func WithStderr(w io.Writer) Option {
	return func(r *Runner) {
		r.Stderr = w
	}
}

// New 创建新的Runner
func New(opts ...Option) *Runner {
	r := &Runner{
		Type:    RunnerTypeExeDocker,
		WorkDir: ".",
		Env:     make(map[string]string),
		Logger:  logrus.WithField("component", "runner"),
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Run 执行流水线
func (r *Runner) Run(ctx context.Context, pipeline *resource.Pipeline) (*ExecutionResult, error) {
	startTime := time.Now()
	result := &ExecutionResult{
		Success:      true,
		StageResults: make([]*StageResult, 0),
	}

	r.Logger.WithFields(logrus.Fields{
		"component": "runner",
	}).Info("开始执行流水线")

	// 准备执行环境
	if err := r.prepareEnvironment(pipeline); err != nil {
		r.Logger.WithError(err).WithFields(logrus.Fields{
			"component": "runner",
		}).Error("准备执行环境失败")
		return result, err
	}

	// 克隆代码源
	if err := r.cloneSources(ctx, pipeline); err != nil {
		r.Logger.WithError(err).WithFields(logrus.Fields{
			"component": "runner",
		}).Error("克隆代码源失败")
		return result, err
	}

	// 执行阶段
	for _, stage := range pipeline.Stages {
		stageResult, err := r.runStage(ctx, pipeline, stage)
		result.StageResults = append(result.StageResults, stageResult)
		if err != nil {
			result.Success = false
			result.Error = err
			break
		}
	}

	result.Duration = time.Since(startTime).Milliseconds()
	r.Logger.WithFields(logrus.Fields{
		"component": "runner",
		"duration":  result.Duration,
	}).Info("流水线执行完成")

	return result, result.Error
}

// RunSpecificJob 执行特定的Job
func (r *Runner) RunSpecificJob(ctx context.Context, pipeline *resource.Pipeline, stageName, jobName string) (*ExecutionResult, error) {
	startTime := time.Now()
	result := &ExecutionResult{
		Success:      true,
		StageResults: make([]*StageResult, 0),
	}

	r.Logger.WithFields(logrus.Fields{
		"component": "runner",
		"stage":     stageName,
		"job":       jobName,
	}).Info("开始执行指定的任务")

	// 准备执行环境
	if err := r.prepareEnvironment(pipeline); err != nil {
		r.Logger.WithError(err).WithFields(logrus.Fields{
			"component": "runner",
		}).Error("准备执行环境失败")
		return result, err
	}

	// 克隆代码源
	if err := r.cloneSources(ctx, pipeline); err != nil {
		r.Logger.WithError(err).WithFields(logrus.Fields{
			"component": "runner",
		}).Error("克隆代码源失败")
		return result, err
	}

	// 查找指定的阶段
	stage, found := pipeline.Stages.Index(stageName)
	if !found {
		err := fmt.Errorf("未找到阶段: %s", stageName)
		r.Logger.WithError(err).WithFields(logrus.Fields{
			"component": "runner",
		}).Error("阶段查找失败")
		return result, err
	}

	// 查找指定的任务
	job, found := stage.Jobs.Index(jobName)
	if !found {
		err := fmt.Errorf("未找到任务: %s", jobName)
		r.Logger.WithError(err).WithFields(logrus.Fields{
			"component": "runner",
		}).Error("任务查找失败")
		return result, err
	}

	// 执行指定的阶段和任务
	stageResult := &StageResult{
		Key:        stage.Key,
		Name:       stage.Name,
		Success:    true,
		JobResults: make([]*JobResult, 0),
	}

	jobResult, err := r.runJob(ctx, pipeline, stage, job)
	stageResult.JobResults = append(stageResult.JobResults, jobResult)
	if err != nil {
		stageResult.Success = false
		stageResult.Error = err
		result.Success = false
		result.Error = err
	}

	result.StageResults = append(result.StageResults, stageResult)

	result.Duration = time.Since(startTime).Milliseconds()
	if result.Success {
		r.Logger.WithFields(logrus.Fields{
			"component": "runner",
			"duration":  result.Duration,
		}).Info("指定任务执行完成")
	} else {
		r.Logger.WithError(result.Error).WithFields(logrus.Fields{
			"component": "runner",
			"duration":  result.Duration,
		}).Error("指定任务执行失败")
	}

	return result, result.Error
}

// prepareEnvironment 准备执行环境
func (r *Runner) prepareEnvironment(pipeline *resource.Pipeline) error {
	r.Logger.WithFields(logrus.Fields{
		"component": "runner",
	}).Info("开始准备执行环境...")

	// 设置Runner级别的环境变量
	for key, value := range r.Env {
		if err := os.Setenv(key, value); err != nil {
			r.Logger.WithError(err).WithFields(logrus.Fields{
				"component": "runner",
				"key":       key,
				"value":     value,
			}).Warn("设置环境变量失败")
		}
	}

	// 设置流水线特定的环境变量
	if pipeline.Variables != nil {
		for key, value := range pipeline.Variables {
			if err := os.Setenv(key, value); err != nil {
				r.Logger.WithError(err).WithFields(logrus.Fields{
					"component": "runner",
					"key":       key,
					"value":     value,
				}).Warn("设置流水线环境变量失败")
			}
		}
	}

	r.Logger.WithFields(logrus.Fields{
		"component": "runner",
	}).Info("执行环境准备完成")
	return nil
}

// cloneSources 克隆所有代码源
func (r *Runner) cloneSources(ctx context.Context, pipeline *resource.Pipeline) error {
	if len(pipeline.Sources) == 0 {
		r.Logger.WithFields(logrus.Fields{
			"component": "runner",
		}).Debug("没有配置代码源，跳过克隆")
		return nil
	}

	r.Logger.WithFields(logrus.Fields{
		"component": "runner",
		"count":     len(pipeline.Sources),
	}).Info("开始克隆代码源...")

	for _, source := range pipeline.Sources {
		if err := r.cloneSource(ctx, source); err != nil {
			return fmt.Errorf("克隆代码源 %s 失败: %w", source.Key, err)
		}
	}

	r.Logger.WithFields(logrus.Fields{
		"component": "runner",
	}).Info("所有代码源克隆完成")
	return nil
}

// cloneSource 克隆代码源
func (r *Runner) cloneSource(ctx context.Context, source *resource.Source) error {
	r.Logger.WithFields(logrus.Fields{
		"component": "runner",
		"source":    source.Key,
		"endpoint":  source.Endpoint,
		"branch":    source.Branch,
	}).Info("开始克隆代码源...")

	// 创建克隆选项
	opts := &scm.CloneOptions{
		URL:    source.Endpoint,
		Branch: source.Branch,
		Commit: source.Commit,
		Depth:  source.CloneDepth,
		Dir:    filepath.Join(r.WorkDir, "sources", source.Key),
	}

	// 如果需要凭证，从服务端获取
	// 优先使用certificate.id，其次是CredentialUID
	credentialUID := ""
	if source.Certificate.ID != "" {
		credentialUID = source.Certificate.ID
	} else if source.CredentialUID != "" {
		credentialUID = source.CredentialUID
	}

	if credentialUID != "" && r.Client != nil {
		r.Logger.WithFields(logrus.Fields{
			"component": "runner",
			"uid":       credentialUID,
		}).Debug("正在获取凭证信息...")

		cred, err := r.Client.GetCredential(ctx, credentialUID)
		if err != nil {
			r.Logger.WithError(err).WithFields(logrus.Fields{
				"component": "runner",
				"uid":       credentialUID,
			}).Error("获取凭证失败")
			return err
		}

		r.Logger.WithFields(logrus.Fields{
			"component": "runner",
			"type":      cred.Type,
			"name":      cred.Name,
		}).Debug("获取凭证成功")

		// 根据凭证类型设置克隆选项
		switch cred.Type {
		case "token":
			opts.Token = cred.Token
			r.Logger.WithFields(logrus.Fields{
				"component": "runner",
			}).Debug("使用Token凭证进行克隆")
		case "password":
			opts.Username = cred.UserName
			opts.Password = cred.Password
			r.Logger.WithFields(logrus.Fields{
				"component": "runner",
			}).Debug("使用用户名密码凭证进行克隆")
		case "ssh":
			opts.PrivateKey = cred.PrivateKey
			r.Logger.WithFields(logrus.Fields{
				"component": "runner",
			}).Debug("使用SSH私钥凭证进行克隆")
		default:
			r.Logger.WithFields(logrus.Fields{
				"component": "runner",
				"type":      cred.Type,
			}).Warn("未知的凭证类型")
		}
	} else {
		r.Logger.WithFields(logrus.Fields{
			"component": "runner",
		}).Debug("未提供凭证信息，将使用匿名方式克隆")
	}

	// 创建SCM克隆器
	cloner := scm.NewCloner(r.Client)

	// 执行克隆
	err := cloner.Clone(ctx, opts)
	if r.OnCloneComplete != nil {
		r.OnCloneComplete(source.Key, err)
	}

	if err != nil {
		r.Logger.WithError(err).WithFields(logrus.Fields{
			"component": "runner",
			"source":    source.Key,
		}).Error("代码源克隆失败")
		return err
	}

	r.Logger.WithFields(logrus.Fields{
		"component": "runner",
		"source":    source.Key,
	}).Info("代码源克隆成功")
	return nil
}

// runStage 执行阶段
func (r *Runner) runStage(ctx context.Context, pipeline *resource.Pipeline, stage *resource.Stage) (*StageResult, error) {
	startTime := time.Now()
	stageResult := &StageResult{
		Key:        stage.Key,
		Name:       stage.Name,
		Success:    true,
		JobResults: make([]*JobResult, 0),
	}

	r.Logger.WithFields(logrus.Fields{
		"component": "runner",
		"stage":     stage.Key,
		"name":      stage.Name,
	}).Info("开始执行阶段")

	// 执行任务
	for _, job := range stage.Jobs {
		jobResult, err := r.runJob(ctx, pipeline, stage, job)
		stageResult.JobResults = append(stageResult.JobResults, jobResult)
		if err != nil {
			if !job.ContinueOnError {
				stageResult.Success = false
				stageResult.Error = err
				break
			}
			r.Logger.WithError(err).WithFields(logrus.Fields{
				"component": "runner",
			}).Warn("任务执行失败但继续执行")
		}
	}

	stageResult.Duration = time.Since(startTime).Milliseconds()
	r.Logger.WithFields(logrus.Fields{
		"component": "runner",
		"stage":     stage.Key,
		"duration":  stageResult.Duration,
	}).Info("阶段执行完成")

	return stageResult, stageResult.Error
}

// runJob 执行任务
func (r *Runner) runJob(ctx context.Context, pipeline *resource.Pipeline, stage *resource.Stage, job *resource.Job) (*JobResult, error) {
	startTime := time.Now()
	jobResult := &JobResult{
		Key:         job.Key,
		Name:        job.Name,
		Success:     true,
		StepResults: make([]*StepResult, 0),
	}

	// 通知任务开始
	if r.OnJobStart != nil {
		r.OnJobStart(stage.Key, job.Key)
	}

	r.Logger.WithFields(logrus.Fields{
		"component": "runner",
		"job":       job.Key,
		"name":      job.Name,
	}).Info("开始执行任务")

	// 选择执行引擎
	eng, err := r.selectEngine(job)
	if err != nil {
		return jobResult, fmt.Errorf("选择执行引擎失败: %w", err)
	}

	// 准备工作目录
	jobWorkDir := filepath.Join(r.WorkDir, stage.Key, job.Key)
	if err := os.MkdirAll(jobWorkDir, 0755); err != nil {
		return jobResult, fmt.Errorf("创建工作目录失败: %w", err)
	}

	// 准备执行选项
	opts := engine.NewOptions()
	opts.WorkDir = jobWorkDir
	opts.Stdout = r.Stdout
	opts.Stderr = r.Stderr
	opts.WithEnvMap(r.Env)

	// 添加流水线全局变量
	if pipeline.Variables != nil {
		opts.WithEnvMap(pipeline.Variables)
	}

	// 添加代码源环境变量
	if src := pipeline.GetDefaultSource(); src != nil {
		opts.WithEnvMap(src.Environ())
	}

	// 添加任务特定环境变量
	if job.Env != nil {
		opts.WithEnvMap(job.Env)
	}

	// 准备执行环境
	if err := eng.Setup(ctx, job, opts); err != nil {
		return jobResult, fmt.Errorf("准备执行环境失败: %w", err)
	}

	// 确保清理执行环境
	defer func() {
		if err := eng.Destroy(context.Background()); err != nil {
			r.Logger.WithError(err).WithFields(logrus.Fields{
				"component": "runner",
			}).Warn("清理执行环境失败")
		}
	}()

	// 执行步骤
	for _, step := range job.Steps {
		stepResult, err := r.runStep(ctx, eng, stage.Key, job.Key, step, opts)
		jobResult.StepResults = append(jobResult.StepResults, stepResult)
		if err != nil {
			if !step.ContinueOnError {
				jobResult.Success = false
				jobResult.Error = err
				break
			}
			r.Logger.WithError(err).WithFields(logrus.Fields{
				"component": "runner",
			}).Warn("步骤执行失败但继续执行")
		}
	}

	jobResult.Duration = time.Since(startTime).Milliseconds()

	// 通知任务完成
	if r.OnJobComplete != nil {
		r.OnJobComplete(stage.Key, job.Key, jobResult.Error)
	}

	if jobResult.Error != nil {
		r.Logger.WithError(jobResult.Error).WithFields(logrus.Fields{
			"component": "runner",
			"job":       job.Key,
		}).Error("任务执行失败")
	} else {
		r.Logger.WithFields(logrus.Fields{
			"component": "runner",
			"job":       job.Key,
			"duration":  jobResult.Duration,
		}).Info("任务执行成功")
	}

	return jobResult, jobResult.Error
}

// runStep 执行步骤
func (r *Runner) runStep(ctx context.Context, eng engine.Engine, stageKey, jobKey string, step *resource.Step, opts *engine.Options) (*StepResult, error) {
	startTime := time.Now()
	stepResult := &StepResult{
		Key:     step.Key,
		Name:    step.Name,
		Success: true,
	}

	// 通知步骤开始
	if r.OnStepStart != nil {
		r.OnStepStart(stageKey, jobKey, step.Key)
	}

	r.Logger.WithFields(logrus.Fields{
		"component": "runner",
		"step":      step.Key,
		"name":      step.Name,
	}).Info("开始执行步骤")

	result, err := eng.Execute(ctx, step, opts)
	if result != nil {
		stepResult.Duration = result.Duration
		stepResult.ExitCode = result.ExitCode
		stepResult.Output = result.Stdout
	}

	// 通知步骤完成
	if r.OnStepComplete != nil {
		r.OnStepComplete(stageKey, jobKey, step.Key, result)
	}

	if err != nil {
		stepResult.Success = false
		return stepResult, err
	}

	stepResult.Duration = time.Since(startTime).Milliseconds()
	r.Logger.WithFields(logrus.Fields{
		"component": "runner",
		"step":      step.Key,
		"duration":  stepResult.Duration,
		"exitCode":  stepResult.ExitCode,
	}).Info("步骤执行完成")

	return stepResult, nil
}

// selectEngine 选择执行引擎
func (r *Runner) selectEngine(job *resource.Job) (engine.Engine, error) {
	// 根据配置或任务类型选择引擎
	var baseEngine engine.Engine
	var err error

	switch r.Type {
	case RunnerTypeDocker:
		baseEngine, err = docker.New()
		if err != nil {
			return nil, err
		}
	case RunnerTypeExe:
		baseEngine = exec.New()
	case RunnerTypeExeDocker:
		// 自动选择: 如果指定了容器镜像则使用Docker,否则使用本地执行
		if job.IsDocker() {
			baseEngine, err = docker.New()
			if err != nil {
				return nil, err
			}
		} else {
			baseEngine = exec.New()
		}
	default:
		return nil, fmt.Errorf("未知的执行器类型: %s", r.Type)
	}

	// 使用适配器包装引擎以支持特殊步骤类型
	adapter := adapter.NewEngineAdapter(baseEngine, r.Logger)
	return adapter, nil
}

// RunFromFile 从文件运行流水线
func (r *Runner) RunFromFile(ctx context.Context, filePath string) (*ExecutionResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取流水线文件失败: %w", err)
	}

	pipeline, err := resource.ParseYaml(data)
	if err != nil {
		return nil, fmt.Errorf("解析流水线失败: %w", err)
	}

	return r.Run(ctx, pipeline)
}
