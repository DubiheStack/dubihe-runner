// Package exec 实现本地执行引擎
package exec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/DubiheStack/dubihe-runner/engine"
	"github.com/DubiheStack/dubihe-runner/resource"
	"github.com/DubiheStack/dubihe-runner/utils"
	"github.com/sirupsen/logrus"
)

// Engine 本地执行引擎
type Engine struct {
	workDir string
	env     map[string]string
	log     *logrus.Entry
	// 添加代码源目录字段，用于在执行步骤时引用
	codeSourceDir string
}

// New 创建本地执行引擎
func New() *Engine {
	return &Engine{
		env: make(map[string]string),
		log: logrus.WithField("engine", "exec"),
	}
}

// Name 返回引擎名称
func (e *Engine) Name() string {
	return "exec"
}

// Ping 检查引擎状态
func (e *Engine) Ping(ctx context.Context) error {
	return nil // 本地执行始终可用
}

// Setup 准备执行环境
func (e *Engine) Setup(ctx context.Context, job *resource.Job, opts *engine.Options) error {
	e.log.WithField("job", job.Key).Info("Setting up exec environment")

	// 设置工作目录
	if opts != nil && opts.WorkDir != "" {
		e.workDir = opts.WorkDir
	} else {
		// 使用临时目录
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
	e.env["DUBIHE_WORKSPACE"] = e.workDir
	e.env["DUBIHE_JOB_NAME"] = job.Name
	e.env["DUBIHE_JOB_KEY"] = job.Key
	
	// 添加WORKSPACE环境变量
	e.env["WORKSPACE"] = e.workDir

	// 尝试查找代码源目录
	// 尝试在工作目录的父目录中查找sources目录
	// 由于工作目录是临时目录，我们需要查找当前工作目录的sources目录
	if codeSourceDir := utils.FindCodeSourceDir("./workspace"); codeSourceDir != "" {
		e.codeSourceDir = codeSourceDir
		e.log.WithField("codeSourceDir", codeSourceDir).Debug("Found code source directory in workspace")
	} else {
		// 如果在workspace目录中没有找到，尝试在当前目录的父目录中查找
		if codeSourceDir := utils.FindCodeSourceDir("."); codeSourceDir != "" {
			e.codeSourceDir = codeSourceDir
			e.log.WithField("codeSourceDir", codeSourceDir).Debug("Found code source directory in current directory")
		}
	}

	e.log.WithField("workDir", e.workDir).Debug("Exec environment ready")
	return nil
}

// Execute 执行步骤
func (e *Engine) Execute(ctx context.Context, step *resource.Step, opts *engine.Options) (*engine.StepResult, error) {
	e.log.WithField("step", step.Key).WithField("name", step.Name).Info("Executing step")

	startTime := time.Now()
	result := &engine.StepResult{}

	// 获取脚本内容
	script := step.GetScript()
	if script == "" {
		return result, nil
	}

	// 获取shell类型
	shell := step.GetShell()
	var cmd *exec.Cmd

	switch shell {
	case "bash":
		cmd = exec.CommandContext(ctx, "bash", "-c", script)
	case "sh":
		cmd = exec.CommandContext(ctx, "sh", "-c", script)
	case "pwsh", "powershell":
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "powershell", "-Command", script)
		} else {
			cmd = exec.CommandContext(ctx, "pwsh", "-Command", script)
		}
	case "cmd":
		cmd = exec.CommandContext(ctx, "cmd", "/c", script)
	default:
		// 默认根据操作系统选择
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "cmd", "/c", script)
		} else {
			cmd = exec.CommandContext(ctx, "bash", "-c", script)
		}
	}

	// 设置工作目录
	workDir := e.workDir
	if step.WorkingDirectory != "" {
		workDir = step.WorkingDirectory
	}
	if opts != nil && opts.WorkDir != "" {
		workDir = opts.WorkDir
	}
	
	// 智能检测是否需要切换到代码源目录
	// 如果脚本包含maven、gradle、npm等命令，尝试使用已找到的代码源目录
	if containsBuildCommand(script) && e.codeSourceDir != "" {
		e.log.WithField("codeSourceDir", e.codeSourceDir).Debug("Using code source directory as working directory for build command")
		workDir = e.codeSourceDir
	}
	cmd.Dir = workDir

	// 设置环境变量
	// 创建环境变量映射，确保正确的覆盖优先级
	envMap := make(map[string]string)

	// 首先添加系统环境变量
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// 然后添加引擎环境变量（会覆盖系统变量）
	for k, v := range e.env {
		envMap[k] = v
	}

	// 添加步骤环境变量（会覆盖前面的同名变量）
	if step.Env != nil {
		for k, v := range step.Env {
			envMap[k] = v
		}
	}

	// 添加选项环境变量（会覆盖前面的同名变量）
	if opts != nil && opts.Env != nil {
		for k, v := range opts.Env {
			envMap[k] = v
		}
	}

	
	// 转换为命令行环境变量格式
	cmd.Env = make([]string, 0, len(envMap))
	for k, v := range envMap {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// 设置步骤环境变量
	cmd.Env = append(cmd.Env, fmt.Sprintf("DUBIHE_STEP_NAME=%s", step.Name))
	cmd.Env = append(cmd.Env, fmt.Sprintf("DUBIHE_STEP_KEY=%s", step.Key))

	// 设置输出
	var stdout, stderr bytes.Buffer
	if opts != nil && opts.Stdout != nil {
		cmd.Stdout = io.MultiWriter(&stdout, opts.Stdout)
	} else {
		cmd.Stdout = io.MultiWriter(&stdout, os.Stdout)
	}
	if opts != nil && opts.Stderr != nil {
		cmd.Stderr = io.MultiWriter(&stderr, opts.Stderr)
	} else {
		cmd.Stderr = io.MultiWriter(&stderr, os.Stderr)
	}

	// 执行命令
	e.log.WithField("script", truncateString(script, 100)).Debug("Running script")
	err := cmd.Run()

	result.Duration = time.Since(startTime).Milliseconds()
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
		if !step.ContinueOnError {
			return result, fmt.Errorf("step %s failed with exit code %d: %w", step.Key, result.ExitCode, err)
		}
		e.log.WithField("step", step.Key).WithField("exitCode", result.ExitCode).Warn("Step failed but continuing")
	}

	e.log.WithField("step", step.Key).WithField("duration", result.Duration).Info("Step completed")
	return result, nil
}

// containsBuildCommand 检查脚本是否包含构建命令
func containsBuildCommand(script string) bool {
	buildCommands := []string{
		"mvn",      // Maven
		"gradle",   // Gradle
		"npm",      // NPM
		"yarn",     // Yarn
		"go build", // Go build
		"go test",  // Go test
		"make",     // Make
		"ant",      // Ant
		"lein",     // Leiningen
		"cargo",    // Cargo (Rust)
	}
	
	scriptLower := strings.ToLower(script)
	for _, cmd := range buildCommands {
		if strings.Contains(scriptLower, strings.ToLower(cmd)) {
			return true
		}
	}
	return false
}

// Destroy 清理执行环境
func (e *Engine) Destroy(ctx context.Context) error {
	e.log.Info("Destroying exec environment")
	// 如果是临时目录，清理
	if strings.Contains(e.workDir, "dubihe-") {
		return os.RemoveAll(e.workDir)
	}
	return nil
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}