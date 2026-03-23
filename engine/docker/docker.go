// Package docker 实现Docker执行引擎
package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/DubiheStack/dubihe-runner/engine"
	"github.com/DubiheStack/dubihe-runner/resource"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/sirupsen/logrus"
)

// Engine Docker执行引擎
type Engine struct {
	client      *client.Client
	containerID string
	image       string
	workDir     string
	env         map[string]string
	log         *logrus.Entry
}

// New 创建Docker执行引擎
func New() (engine.Engine, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &Engine{
		client: cli,
		env:    make(map[string]string),
		log:    logrus.WithField("engine", "docker"),
	}, nil
}

// Name 返回引擎名称
func (e *Engine) Name() string {
	return "docker"
}

// Ping 检查引擎状态
func (e *Engine) Ping(ctx context.Context) error {
	_, err := e.client.Ping(ctx)
	return err
}

// Setup 准备执行环境
func (e *Engine) Setup(ctx context.Context, job *resource.Job, opts *engine.Options) error {
	e.log.WithField("job", job.Key).Info("Setting up docker environment")

	// 获取镜像
	e.image = job.RunsOn.Container
	if e.image == "" {
		return fmt.Errorf("container image is required for docker engine")
	}

	// 拉取镜像
	e.log.WithField("image", e.image).Info("Pulling docker image")
	reader, err := e.client.ImagePull(ctx, e.image, types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", e.image, err)
	}
	defer reader.Close()
	io.Copy(io.Discard, reader)

	// 设置工作目录
	if opts != nil && opts.WorkDir != "" {
		e.workDir = opts.WorkDir
	} else {
		tmpDir, err := os.MkdirTemp("", "dubihe-*")
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		e.workDir = tmpDir
	}

	// 合并环境变量
	if opts != nil && opts.Env != nil {
		for k, v := range opts.Env {
			e.env[k] = v
		}
	}
	if job.Env != nil {
		for k, v := range job.Env {
			e.env[k] = v
		}
	}

	// 设置默认环境变量
	e.env["DUBIHE_WORKSPACE"] = "/workspace"
	e.env["DUBIHE_JOB_NAME"] = job.Name
	e.env["DUBIHE_JOB_KEY"] = job.Key
	
	// 添加WORKSPACE环境变量
	e.env["WORKSPACE"] = "/workspace"

	// 准备环境变量列表
	envList := make([]string, 0, len(e.env))
	for k, v := range e.env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}

	// 创建容器
	containerConfig := &container.Config{
		Image:      e.image,
		Env:        envList,
		WorkingDir: "/workspace",
		Tty:        true,
		Cmd:        []string{"tail", "-f", "/dev/null"}, // 保持容器运行
	}

	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: e.workDir,
				Target: "/workspace",
			},
		},
	}

	// 添加服务容器网络配置
	resp, err := e.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	e.containerID = resp.ID

	// 启动容器
	if err := e.client.ContainerStart(ctx, e.containerID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	e.log.WithField("containerID", e.containerID[:12]).Debug("Docker container started")
	return nil
}

// Execute 执行步骤
func (e *Engine) Execute(ctx context.Context, step *resource.Step, opts *engine.Options) (*engine.StepResult, error) {
	e.log.WithField("step", step.Key).WithField("name", step.Name).Info("Executing step in container")

	startTime := time.Now()
	result := &engine.StepResult{}

	// 获取脚本内容
	script := step.GetScript()
	if script == "" {
		return result, nil
	}

	// 获取shell
	shell := step.GetShell()

	// 准备命令
	var cmd []string
	switch shell {
	case "bash":
		cmd = []string{"bash", "-c", script}
	case "sh":
		cmd = []string{"sh", "-c", script}
	default:
		cmd = []string{"bash", "-c", script}
	}

	// 准备环境变量
	envList := make([]string, 0)
	envList = append(envList, fmt.Sprintf("DUBIHE_STEP_NAME=%s", step.Name))
	envList = append(envList, fmt.Sprintf("DUBIHE_STEP_KEY=%s", step.Key))
	if step.Env != nil {
		for k, v := range step.Env {
			envList = append(envList, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// 设置工作目录
	workDir := "/workspace"
	if step.WorkingDirectory != "" {
		workDir = step.WorkingDirectory
	}

	// 创建exec配置
	execConfig := types.ExecConfig{
		Cmd:          cmd,
		Env:          envList,
		WorkingDir:   workDir,
		AttachStdout: true,
		AttachStderr: true,
	}

	// 创建exec实例
	execResp, err := e.client.ContainerExecCreate(ctx, e.containerID, execConfig)
	if err != nil {
		return result, fmt.Errorf("failed to create exec: %w", err)
	}

	// 附加到exec
	attachResp, err := e.client.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
	if err != nil {
		return result, fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attachResp.Close()

	// 读取输出
	var stdout, stderr bytes.Buffer
	if opts != nil && opts.Stdout != nil {
		stdcopy.StdCopy(io.MultiWriter(&stdout, opts.Stdout), io.MultiWriter(&stderr, opts.Stderr), attachResp.Reader)
	} else {
		stdcopy.StdCopy(io.MultiWriter(&stdout, os.Stdout), io.MultiWriter(&stderr, os.Stderr), attachResp.Reader)
	}

	// 获取退出码
	inspectResp, err := e.client.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return result, fmt.Errorf("failed to inspect exec: %w", err)
	}

	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	result.ExitCode = inspectResp.ExitCode
	result.Duration = time.Since(startTime).Milliseconds()

	if result.ExitCode != 0 {
		if !step.ContinueOnError {
			return result, fmt.Errorf("step %s failed with exit code %d", step.Key, result.ExitCode)
		}
		e.log.WithField("step", step.Key).WithField("exitCode", result.ExitCode).Warn("Step failed but continuing")
	}

	e.log.WithField("step", step.Key).WithField("duration", result.Duration).Info("Step completed")
	return result, nil
}

// Destroy 清理执行环境
func (e *Engine) Destroy(ctx context.Context) error {
	e.log.Info("Destroying docker environment")
	
	// 停止容器
	if e.containerID != "" {
		timeout := 10
		if err := e.client.ContainerStop(ctx, e.containerID, container.StopOptions{Timeout: &timeout}); err != nil {
			e.log.WithError(err).Warn("Failed to stop container")
		}

		// 删除容器
		if err := e.client.ContainerRemove(ctx, e.containerID, types.ContainerRemoveOptions{Force: true}); err != nil {
			e.log.WithError(err).Warn("Failed to remove container")
		}
	}

	// 清理工作目录
	if strings.Contains(e.workDir, "dubihe-") {
		return os.RemoveAll(e.workDir)
	}
	return nil
}