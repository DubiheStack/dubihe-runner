// Package cmd 提供云效Runner命令行接口
package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/DubiheStack/dubihe-runner/client"
	"github.com/DubiheStack/dubihe-runner/daemon" // 添加daemon导入
	"github.com/DubiheStack/dubihe-runner/engine"
	"github.com/DubiheStack/dubihe-runner/monitor"
	"github.com/DubiheStack/dubihe-runner/resource"
	"github.com/DubiheStack/dubihe-runner/runner"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	version = "1.0.0"
)

var rootCmd = &cobra.Command{
	Use:   "dubihe",
	Short: "天枢Runner - 流水线执行引擎",
	Long: `云效Runner是一个兼容阿里云云效Flow的流水线执行引擎。

支持以下功能:
  - 本地执行 (exec) 和 Docker 容器执行
  - 多阶段、多任务、多步骤的流水线
  - 环境变量和变量替换
  - 条件执行和错误处理
  - 制品上传和缓存

示例:
  dubihe run -f pipeline.yaml
  dubihe run -f pipeline.yaml --engine docker
  dubihe validate -f pipeline.yaml`,
	Version: version,
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "运行流水线",
	Long:  "执行指定的流水线配置文件",
	RunE:  runPipeline,
}

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "以守护进程模式运行",
	Long:  "以守护进程模式运行Runner，在后台持续发送心跳并拉取执行任务",
	RunE:  runDaemon,
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "验证流水线配置",
	Long:  "验证流水线配置文件的语法和结构",
	RunE:  validatePipeline,
}

var parseCmd = &cobra.Command{
	Use:   "parse",
	Short: "解析流水线配置",
	Long:  "解析并输出流水线配置的结构化信息",
	RunE:  parsePipeline,
}

var (
	pipelineFile string
	engineType   string
	workDir      string
	verbose      bool
	envVars      []string
	configFile   string
	skipVerify   bool
	daemonMode   bool
	logFile      string

	// heartbeatMgr 心跳管理器
	heartbeatMgr *client.HeartbeatManager
	// runnerConfig 运行时配置
	runnerConfig *client.Config
	// apiClient API客户端
	c *client.Client
)

// 新增一个变量用于标记是否处于开发模式
var devMode = false

// 保存原始的日志输出，以便在需要时恢复
var originalOutput = os.Stdout

func init() {
	// 全局标志
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "启用详细输出")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.yaml", "Runner配置文件路径")
	rootCmd.PersistentFlags().BoolVar(&skipVerify, "skip-verify", false, "跳过TLS证书验证")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "日志文件路径")

	// run 命令标志
	runCmd.Flags().StringVarP(&pipelineFile, "file", "f", "", "流水线配置文件路径 (必填)")
	runCmd.Flags().StringVarP(&engineType, "engine", "e", "exe_docker", "执行引擎类型 (exe, docker, exe_docker)")
	runCmd.Flags().StringVarP(&workDir, "workdir", "w", "", "工作目录")
	runCmd.Flags().StringArrayVar(&envVars, "env", nil, "环境变量 (格式: KEY=VALUE)")
	runCmd.MarkFlagRequired("file")

	// validate 命令标志
	validateCmd.Flags().StringVarP(&pipelineFile, "file", "f", "", "流水线配置文件路径 (必填)")
	validateCmd.MarkFlagRequired("file")

	// parse 命令标志
	parseCmd.Flags().StringVarP(&pipelineFile, "file", "f", "", "流水线配置文件路径 (必填)")
	parseCmd.MarkFlagRequired("file")

	// 添加子命令
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(parseCmd)
}

// 自定义日志格式化器
type CustomFormatter struct {
	EnableColors bool
}

// Format 实现 logrus.Formatter 接口
func (f *CustomFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	// 时间戳
	b.WriteString(entry.Time.Format("2006-01-02 15:04:05.000"))

	// 日志级别
	switch entry.Level {
	case logrus.DebugLevel:
		b.WriteString(" DEBUG ")
	case logrus.InfoLevel:
		b.WriteString(" INFO  ")
	case logrus.WarnLevel:
		b.WriteString(" WARN  ")
	case logrus.ErrorLevel:
		b.WriteString(" ERROR ")
	case logrus.FatalLevel:
		b.WriteString(" FATAL ")
	case logrus.PanicLevel:
		b.WriteString(" PANIC ")
	}

	// 组件信息（如果存在）
	if component, ok := entry.Data["component"]; ok {
		b.WriteString(fmt.Sprintf("[%s] ", component))
		delete(entry.Data, "component") // 从Data中移除，避免重复显示
	}

	// 消息内容
	b.WriteString(entry.Message)

	// 其他字段（除了已经处理过的component）
	hasFields := false
	for k, v := range entry.Data {
		if !hasFields {
			b.WriteString(" ")
			hasFields = true
		}
		b.WriteString(fmt.Sprintf("%s=%v ", k, v))
	}

	// 换行
	b.WriteString("\n")

	// 如果启用颜色，添加颜色代码
	if f.EnableColors {
		switch entry.Level {
		case logrus.DebugLevel:
			return []byte(fmt.Sprintf("\x1b[36m%s\x1b[0m", b.String())), nil // 青色
		case logrus.InfoLevel:
			return []byte(fmt.Sprintf("\x1b[32m%s\x1b[0m", b.String())), nil // 绿色
		case logrus.WarnLevel:
			return []byte(fmt.Sprintf("\x1b[33m%s\x1b[0m", b.String())), nil // 黄色
		case logrus.ErrorLevel:
			return []byte(fmt.Sprintf("\x1b[31m%s\x1b[0m", b.String())), nil // 红色
		default:
			return b.Bytes(), nil
		}
	}

	return b.Bytes(), nil
}

// Execute 执行命令
func Execute() error {
	// 保存原始输出
	originalOutput = os.Stdout

	// 设置日志格式，使用年月日 时分秒毫米格式
	formatter := &CustomFormatter{
		EnableColors: false, // 默认禁用颜色
	}

	logrus.SetFormatter(formatter)

	// 在执行命令前进行注册
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// 设置日志级别
		if verbose {
			logrus.SetLevel(logrus.DebugLevel)
		} else {
			logrus.SetLevel(logrus.InfoLevel)
		}

		// 从配置文件加载配置
		cfg, err := client.LoadConfig(configFile)
		if err != nil {
			return fmt.Errorf("加载配置文件失败: %w", err)
		}

		// 设置开发模式
		if cfg.DevMode {
			devMode = true
		} else {
			devMode = os.Getenv("DUBIHE_DEV_MODE") == "true"
		}

		// 如果命令行指定了日志文件，则使用命令行参数，否则使用配置文件中的设置
		effectiveLogFile := logFile
		if effectiveLogFile == "" {
			// 首先检查配置根级别的LogFile字段
			if cfg.LogFile != "" {
				effectiveLogFile = cfg.LogFile
			} else if cfg.LogConfig != nil && cfg.LogConfig.LogFile != "" {
				// 然后检查LogConfig中的LogFile字段
				effectiveLogFile = cfg.LogConfig.LogFile
			}
		}

		// 无论是否为开发模式，都尝试将日志输出到指定文件
		var finalLogFile string
		if effectiveLogFile != "" {
			// 如果指定了日志文件，则将日志输出重定向到文件
			finalLogFile = setupLogFile(effectiveLogFile)
		} else {
			// 如果没有指定日志文件，使用默认日志文件路径
			finalLogFile = setupLogFile("")
		}

		// 打开日志文件
		file, err := os.OpenFile(finalLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			logrus.WithError(err).Warnf("打开日志文件失败: %s", finalLogFile)
			// 如果还是失败，则输出到标准输出
			logrus.SetOutput(os.Stdout)
		} else {
			// 设置日志输出到文件，并确保禁用颜色
			formatter.EnableColors = false

			// 无论是否为开发模式，都将日志输出到文件
			if devMode {
				// 在开发模式下，同时输出到文件和控制台
				multiWriter := io.MultiWriter(file, os.Stdout)
				logrus.SetOutput(multiWriter)
				formatter.EnableColors = true
				logrus.Infof("[System] 日志将输出到文件: %s 和控制台", finalLogFile)
			} else {
				// 在非开发模式下，只输出到文件
				logrus.SetOutput(file)
				logrus.Infof("[System] 日志将输出到文件: %s", finalLogFile)
			}
		}

		// 保存配置以供其他地方使用
		runnerConfig = cfg

		// 注册Runner
		if err := registerRunner(); err != nil {
			logrus.WithError(err).Warn("[Runner] 注册Runner失败，继续执行...")
		}
		return nil
	}

	return rootCmd.Execute()
}

// registerRunner 从配置文件读取配置并向服务端注册Runner
func registerRunner() error {
	logrus.WithFields(logrus.Fields{
		"configFile": configFile,
		"url":        runnerConfig.Url,
		"uid":        runnerConfig.Uid,
		"type":       runnerConfig.Type,
		"workDir":    runnerConfig.GetWorkDir(),
		"logType":    runnerConfig.GetLogConfig().LogType,
		"logFile":    logFile,
		"devMode":    devMode,
	}).Info("已加载配置文件")

	// 初始化客户端
	c = client.New(runnerConfig.Url, runnerConfig.Token, skipVerify)
	info := runnerConfig.ToRunnerMachine()

	// 尝试注册Runner
	registerErr := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return c.Register(ctx, info)
	}()

	// 无论注册是否成功，都启动心跳任务和资源监控
	heartbeatMgr = client.NewHeartbeatManager(c, info, runnerConfig.GetHeartbeatInterval())
	heartbeatMgr.Start()

	// 启动资源监控任务
	startResourceMonitoring()

	if registerErr != nil {
		logrus.WithError(registerErr).WithFields(logrus.Fields{
			"component": "runner",
		}).Warn("注册Runner失败，继续执行...")
		return registerErr
	}

	logrus.WithFields(logrus.Fields{
		"component": "runner",
	}).Info("Runner注册成功")
	return nil
}

// setupLogFile 设置日志文件路径，确保目录存在且可以写入
func setupLogFile(logFile string) string {
	var targetFile string

	if logFile != "" {
		// 使用指定的日志文件
		targetFile = logFile
	} else {
		// 使用默认日志文件
		targetFile = "./workspace/log/dubihe-runner.log"
	}

	// 确保日志目录存在
	logDir := filepath.Dir(targetFile)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		// 如果无法创建指定目录，尝试在当前目录创建
		targetFile = filepath.Join(".", "log", filepath.Base(targetFile))
		logDir = filepath.Dir(targetFile)
		os.MkdirAll(logDir, 0755)
	} else if !canWriteToFile(targetFile) {
		// 如果无法写入指定文件，尝试在当前目录创建
		targetFile = filepath.Join(".", "log", filepath.Base(targetFile))
		logDir = filepath.Dir(targetFile)
		os.MkdirAll(logDir, 0755)
	}

	return targetFile
}

// canWriteToFile 检查文件是否可以写入
func canWriteToFile(filename string) bool {
	// 尝试创建或打开文件进行测试
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

// stopHeartbeat 停止心跳任务
func stopHeartbeat() {
	if heartbeatMgr != nil {
		heartbeatMgr.Stop()
	}
}

// runDaemon 以守护进程模式运行
func runDaemon(cmd *cobra.Command, args []string) error {
	logrus.WithFields(logrus.Fields{
		"component": "daemon",
	}).Info("以守护进程模式启动Runner")

	// 创建守护进程实例（不再使用pollingFunc，daemon内部直接调用PullJobData）
	d := daemon.New(c, runnerConfig, nil)

	// 启动守护进程
	d.Start()

	// 创建上下文，支持取消
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 确保退出时停止心跳和守护进程
	defer func() {
		stopHeartbeat()
		d.Stop()
	}()

	// 监听中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		logrus.WithFields(logrus.Fields{
			"component": "daemon",
		}).Info("收到中断信号，正在停止守护进程...")
		cancel()
	}()

	// 在开发模式下，定期输出心跳信息到控制台
	if devMode {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					logrus.WithFields(logrus.Fields{
						"component": "daemon",
						"status":    "running",
					}).Info("守护进程心跳")
				case <-ctx.Done():
					return
				}
			}
		}()
	} else {
		// 在非开发模式下，定期输出心跳信息到日志文件
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					logrus.WithFields(logrus.Fields{
						"component": "daemon",
						"status":    "running",
					}).Debug("守护进程心跳")
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// 保持运行状态
	<-ctx.Done()
	logrus.WithFields(logrus.Fields{
		"component": "daemon",
	}).Info("守护进程已停止")

	return nil
}

// runPipeline 运行流水线
func runPipeline(cmd *cobra.Command, args []string) error {
	// 解析环境变量
	env := make(map[string]string)
	for _, e := range envVars {
		var key, value string
		if n := len(e); n > 0 {
			for i := 0; i < n; i++ {
				if e[i] == '=' {
					key = e[:i]
					value = e[i+1:]
					break
				}
			}
		}
		if key != "" {
			env[key] = value
		}
	}

	// 确定执行器类型
	var runnerType runner.RunnerType
	switch engineType {
	case "docker":
		runnerType = runner.RunnerTypeDocker
	case "exe":
		runnerType = runner.RunnerTypeExe
	case "exe_docker":
		runnerType = runner.RunnerTypeExeDocker
	default:
		runnerType = runner.RunnerTypeExeDocker
	}

	// 确定工作目录：命令行优先，否则使用配置文件
	actualWorkDir := workDir
	if actualWorkDir == "" && runnerConfig != nil {
		actualWorkDir = runnerConfig.GetWorkDir()
	}

	// 创建Runner
	r := runner.New(
		runner.WithType(runnerType),
		runner.WithWorkDir(actualWorkDir),
		runner.WithEnv(env),
		runner.WithClient(c), // 传递API客户端用于获取凭证
	)

	// 启动资源监控
	startResourceMonitoring()

	// 设置回调
	r.OnStepStart = func(stage, job, step string) {
		logrus.WithFields(logrus.Fields{
			"stage": stage,
			"job":   job,
			"step":  step,
		}).Info("[Step] ▶ 开始执行步骤")

		// 上报资源使用情况
		reportResourceUsage()
	}

	r.OnStepComplete = func(stage, job, step string, result *engine.StepResult) {
		fields := logrus.Fields{
			"stage":    stage,
			"job":      job,
			"step":     step,
			"duration": fmt.Sprintf("%dms", result.Duration),
		}
		if result.ExitCode != 0 {
			fields["exitCode"] = result.ExitCode
			logrus.WithFields(fields).Warn("[Step] ✗ 步骤执行失败")
		} else {
			logrus.WithFields(fields).Info("[Step] ✓ 步骤执行成功")
		}

		// 上报资源使用情况
		reportResourceUsage()
	}

	r.OnJobStart = func(stage, job string) {
		logrus.WithFields(logrus.Fields{
			"stage": stage,
			"job":   job,
		}).Info("[Job] ━━━ 开始执行任务 ━━━")

		// 上报资源使用情况
		reportResourceUsage()
	}

	r.OnJobComplete = func(stage, job string, err error) {
		fields := logrus.Fields{
			"stage": stage,
			"job":   job,
		}
		if err != nil {
			logrus.WithFields(fields).WithError(err).Error("[Job] ━━━ 任务执行失败 ━━━")
		} else {
			logrus.WithFields(fields).Info("[Job] ━━━ 任务执行完成 ━━━")
		}

		// 上报资源使用情况
		reportResourceUsage()
	}

	// 创建上下文，支持取消
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 确保退出时停止心跳
	defer stopHeartbeat()

	// 监听中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		logrus.Warn("[System] 收到中断信号，正在取消执行...")
		cancel()
	}()

	// 执行流水线
	logrus.WithField("file", pipelineFile).Info("[Pipeline] 开始执行流水线")
	result, err := r.RunFromFile(ctx, pipelineFile)

	if err != nil {
		logrus.WithError(err).Error("[Pipeline] 流水线执行失败")
		return err
	}

	// 输出执行结果
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════")
	fmt.Printf("执行结果: %s\n", statusStr(result.Success))
	fmt.Printf("总耗时: %dms\n", result.Duration)
	fmt.Println("═══════════════════════════════════════════════════")

	for _, stageResult := range result.StageResults {
		fmt.Printf("\n阶段: %s (%s) - %s\n", stageResult.Name, stageResult.Key, statusStr(stageResult.Success))
		for _, jobResult := range stageResult.JobResults {
			fmt.Printf("  任务: %s (%s) - %s [%dms]\n", jobResult.Name, jobResult.Key, statusStr(jobResult.Success), jobResult.Duration)
			for _, stepResult := range jobResult.StepResults {
				fmt.Printf("    步骤: %s (%s) - %s [%dms]\n", stepResult.Name, stepResult.Key, statusStr(stepResult.Success), stepResult.Duration)
			}
		}
	}

	if !result.Success {
		return fmt.Errorf("pipeline execution failed")
	}

	return nil
}

// validatePipeline 验证流水线配置
func validatePipeline(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(pipelineFile)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	pipeline, err := resource.ParseYaml(data)
	if err != nil {
		return fmt.Errorf("配置文件验证失败: %w", err)
	}

	fmt.Println("✓ 配置文件验证通过")
	fmt.Printf("  流水线名称: %s\n", pipeline.Name)
	fmt.Printf("  阶段数量: %d\n", len(pipeline.Stages))

	totalJobs := 0
	totalSteps := 0
	for _, stage := range pipeline.Stages {
		totalJobs += len(stage.Jobs)
		for _, job := range stage.Jobs {
			totalSteps += len(job.Steps)
		}
	}

	fmt.Printf("  任务数量: %d\n", totalJobs)
	fmt.Printf("  步骤数量: %d\n", totalSteps)

	return nil
}

// parsePipeline 解析流水线配置
func parsePipeline(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(pipelineFile)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	pipeline, err := resource.ParseYaml(data)
	if err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 输出结构化信息
	output, err := yaml.Marshal(pipeline)
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}

	fmt.Println("解析后的流水线结构:")
	fmt.Println("---")
	fmt.Println(string(output))

	return nil
}

// reportResourceUsage 上报资源使用情况
func reportResourceUsage() {
	if c == nil || runnerConfig == nil {
		return
	}

	// 收集资源使用情况
	usage, err := monitor.Collect(runnerConfig.HostUid)
	if err != nil {
		logrus.WithError(err).Debug("[Resource] 收集资源使用情况失败")
		return
	}

	// 上报资源使用情况
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = c.ReportResourceUsage(ctx, &client.ResourceUsage{
		CPU:    usage.CPU,
		Mem:    usage.Mem,
		Disk:   usage.Disk,
		HostID: usage.HostID,
	})
	if err != nil {
		logrus.WithError(err).Debug("[Resource] 资源使用情况上报失败")
	} else {
		logrus.WithFields(logrus.Fields{
			"cpu":    usage.CPU,
			"memory": usage.Mem,
			"disk":   usage.Disk,
		}).Info("[Resource] 资源使用情况上报成功")
	}
}

// startResourceMonitoring 启动资源监控任务
func startResourceMonitoring() {
	if c == nil || runnerConfig == nil {
		logrus.Debug("[Resource] 无法启动资源监控：客户端或配置为空")
		return
	}

	// 启动一个goroutine定期上报资源使用情况
	go func() {
		// 每隔一定时间上报一次资源使用情况
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				reportResourceUsage()
			}
		}
	}()

	logrus.Info("[Resource] 资源监控任务已启动，每30秒上报一次")
}

// DualLogHook 实现同时向控制台和文件输出日志的钩子
type DualLogHook struct {
	formatter *CustomFormatter
}

// Levels 返回此钩子适用的日志级别
func (hook *DualLogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire 实现日志钩子接口
func (hook *DualLogHook) Fire(entry *logrus.Entry) error {
	formatted, err := hook.formatter.Format(entry)
	if err != nil {
		return err
	}
	fmt.Print(string(formatted))
	return nil
}

func statusStr(success bool) string {
	if success {
		return "✓ 成功"
	}
	return "✗ 失败"
}
