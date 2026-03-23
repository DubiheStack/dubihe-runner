// Package daemon 实现Runner的守护进程功能
package daemon

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DubiheStack/dubihe-runner/client"
	"github.com/DubiheStack/dubihe-runner/engine"
	"github.com/DubiheStack/dubihe-runner/license"
	"github.com/DubiheStack/dubihe-runner/resource"
	"github.com/DubiheStack/dubihe-runner/runner"
	"github.com/sirupsen/logrus"
)

// Daemon 守护进程
type Daemon struct {
	client      *client.Client
	config      *client.Config
	runners     []*runner.Runner
	logger      *logrus.Entry
	ctx         context.Context
	cancel      context.CancelFunc
	isRunning   bool
	pollingFunc func() (*resource.Pipeline, error)
	concurrency int
	activeTasks map[string]*runner.ExecutionResult
	taskMutex   sync.RWMutex
	autoUpgrade bool
}

// New 创建新的守护进程
func New(c *client.Client, config *client.Config, pollingFunc func() (*resource.Pipeline, error)) *Daemon {
	ctx, cancel := context.WithCancel(context.Background())

	logger := logrus.WithField("component", "daemon")

	// 创建多个Runner实例以支持并发
	concurrency := config.GetConcurrency()
	runners := make([]*runner.Runner, concurrency)
	for i := 0; i < concurrency; i++ {
		runners[i] = runner.New(
			runner.WithType(runner.RunnerType(config.Type)),
			runner.WithWorkDir(config.GetWorkDir()),
			runner.WithClient(c),
			runner.WithLogger(logger),
		)
	}

	return &Daemon{
		client:      c,
		config:      config,
		runners:     runners,
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
		isRunning:   false,
		pollingFunc: pollingFunc,
		concurrency: concurrency,
		activeTasks: make(map[string]*runner.ExecutionResult),
		autoUpgrade: config.AutoUpgrade,
	}
}

// Start 启动守护进程
func (d *Daemon) Start() {
	if d.isRunning {
		d.logger.Warn("守护进程已在运行中")
		return
	}

	d.isRunning = true
	d.logger.Info("守护进程开始运行")

	// 启动轮询任务
	go d.pollingLoop()

	// 如果启用了自动更新，启动更新检查任务
	if d.autoUpgrade {
		go d.upgradeCheckLoop()
	}
}

// Stop 停止守护进程
func (d *Daemon) Stop() {
	if !d.isRunning {
		d.logger.Warn("守护进程未在运行中")
		return
	}

	d.logger.Info("正在停止守护进程...")
	d.cancel()
	d.isRunning = false
	d.logger.Info("守护进程已停止")
}

// IsRunning 检查守护进程是否正在运行
func (d *Daemon) IsRunning() bool {
	return d.isRunning
}

// pollingLoop 轮询循环
func (d *Daemon) pollingLoop() {
	// 使用配置的扫描间隔作为轮询间隔
	interval := d.config.GetScanInterval()
	if interval <= 0 {
		interval = client.DefaultHeartbeatInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	d.logger.WithFields(logrus.Fields{
		"interval": interval,
	}).Info("轮询任务已启动")

	for {
		select {
		case <-d.ctx.Done():
			d.logger.WithFields(logrus.Fields{
				"status": "stopped",
			}).Info("轮询任务已停止")
			return
		case <-ticker.C:
			d.processJobs()
		}
	}
}

// upgradeCheckLoop 自动更新检查循环
func (d *Daemon) upgradeCheckLoop() {
	if !d.autoUpgrade {
		return
	}

	interval := d.config.GetUpgradeInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	d.logger.WithFields(logrus.Fields{
		"interval": interval,
	}).Info("自动更新检查已启动")

	for {
		select {
		case <-d.ctx.Done():
			d.logger.Info("自动更新检查已停止")
			return
		case <-ticker.C:
			d.checkForUpgrades()
		}
	}
}

// processJobs 处理任务
func (d *Daemon) processJobs() {
	d.logger.WithFields(logrus.Fields{
		"component": "job_processor",
	}).Info("开始检查新任务")

	// 检查许可证是否有效
	if license.IsExpired() {
		d.logger.Error("license已过期，请联系授权方进行license更新。")
		return
	}

	// 检查当前活跃任务数量
	d.taskMutex.RLock()
	activeTaskCount := len(d.activeTasks)
	d.taskMutex.RUnlock()

	// 如果已达到最大并发数，跳过本次轮询
	if activeTaskCount >= d.concurrency {
		d.logger.WithFields(logrus.Fields{
			"active_tasks": activeTaskCount,
			"concurrency":  d.concurrency,
		}).Debug("已达到最大并发任务数，跳过本次轮询")
		return
	}

	// 直接拉取完整的JobData（包含stageName、jobName、payload等）
	jobData, err := d.PullJobData()
	if err != nil {
		d.logger.WithError(err).WithFields(logrus.Fields{
			"component": "job_processor",
		}).Error("拉取任务失败")
		return
	}

	// 如果没有任务，直接返回
	if jobData == nil {
		d.logger.WithFields(logrus.Fields{
			"component": "job_processor",
		}).Debug("暂无新任务")
		return
	}

	// 解析YAML配置为Pipeline对象
	if jobData.YamlConfig == "" {
		d.logger.WithFields(logrus.Fields{
			"component": "job_processor",
		}).Error("任务的YAML配置为空")
		return
	}

	pipeline, err := resource.ParseYaml([]byte(jobData.YamlConfig))
	if err != nil {
		d.logger.WithError(err).WithFields(logrus.Fields{
			"component": "job_processor",
		}).Error("解析任务YAML配置失败")
		return
	}

	// 分配任务给可用的Runner
	availableRunnerIndex := d.getAvailableRunnerIndex()
	if availableRunnerIndex == -1 {
		d.logger.WithFields(logrus.Fields{
			"component": "job_processor",
		}).Warn("没有可用的Runner实例")
		return
	}

	d.logger.WithFields(logrus.Fields{
		"component": "job_processor",
		"pipeline":  pipeline.Name,
		"stage":     jobData.StageName,
		"job":       jobData.JobName,
		"jobUId":    jobData.JobUId,
	}).Info("发现新任务，开始执行")

	// 在单独的goroutine中执行任务
	go d.executeTask(pipeline, jobData, availableRunnerIndex)
}

// getAvailableRunnerIndex 获取可用的Runner索引
func (d *Daemon) getAvailableRunnerIndex() int {
	d.taskMutex.RLock()
	defer d.taskMutex.RUnlock()

	// 简单实现：返回第一个可用的Runner
	// 在更复杂的实现中，可以根据负载均衡策略选择Runner
	if len(d.activeTasks) < d.concurrency {
		return len(d.activeTasks) % d.concurrency
	}

	return -1
}

// executeTask 执行任务
func (d *Daemon) executeTask(pipeline *resource.Pipeline, jobData *client.JobData, runnerIndex int) {
	// 记录任务开始
	taskID := fmt.Sprintf("%s-%d", pipeline.Name, time.Now().Unix())
	d.taskMutex.Lock()
	d.activeTasks[taskID] = nil // 占位符
	d.taskMutex.Unlock()

	defer func() {
		// 清理任务记录
		d.taskMutex.Lock()
		delete(d.activeTasks, taskID)
		d.taskMutex.Unlock()
	}()

	// 设置Runner回调：实时上传日志
	r := d.runners[runnerIndex]
	d.setupRunnerCallbacks(r, jobData)

	// 执行任务
	var result *runner.ExecutionResult
	var err error

	if jobData.StageName != "" && jobData.JobName != "" {
		// 执行特定的stage和job
		d.logger.WithFields(logrus.Fields{
			"component": "task_executor",
			"stage":     jobData.StageName,
			"job":       jobData.JobName,
			"jobUId":    jobData.JobUId,
		}).Info("执行特定的stage和job")
		result, err = r.RunSpecificJob(d.ctx, pipeline, jobData.StageName, jobData.JobName)
	} else {
		// 执行整个流水线（兼容旧版本）
		d.logger.WithFields(logrus.Fields{
			"component": "task_executor",
		}).Info("执行整个流水线（兼容旧版本）")
		result, err = r.Run(d.ctx, pipeline)
	}

	// 上报任务执行结果
	d.reportJobResult(jobData, result, err)

	if err != nil {
		d.logger.WithError(err).WithFields(logrus.Fields{
			"component": "task_executor",
		}).Error("执行任务失败")
		return
	}

	// 记录执行结果
	d.taskMutex.Lock()
	d.activeTasks[taskID] = result
	d.taskMutex.Unlock()

	// 记录执行结果
	if result.Success {
		d.logger.WithFields(logrus.Fields{
			"component": "task_executor",
			"success":   true,
		}).Info("任务执行成功")
	} else {
		d.logger.WithError(result.Error).WithFields(logrus.Fields{
			"component": "task_executor",
			"success":   false,
		}).Error("任务执行失败")
	}

	// 输出详细结果
	d.printExecutionResult(result)
}

// setupRunnerCallbacks 设置Runner的回调函数，实现实时日志上传
func (d *Daemon) setupRunnerCallbacks(r *runner.Runner, jobData *client.JobData) {
	// 构建步骤UID映射（stepName -> StepPayload）
	stepMap := make(map[string]*client.StepPayload)
	for i := range jobData.Payload {
		stepMap[jobData.Payload[i].StepName] = &jobData.Payload[i]
	}

	d.logger.WithFields(logrus.Fields{
		"payloadSize": len(jobData.Payload),
		"stepMapSize": len(stepMap),
		"jobUId":      jobData.JobUId,
		"jobExecUid":  jobData.JobExecutionUid,
	}).Info("[回调设置] 步骤Payload映射构建完成")
	for name, p := range stepMap {
		d.logger.WithFields(logrus.Fields{
			"stepName":  name,
			"detailUid": p.DetailUid,
			"stepUid":   p.StepUid,
		}).Debug("[回调设置] 步骤映射详情")
	}

	r.OnStepStart = func(stage, job, step string) {
		d.logger.WithFields(logrus.Fields{
			"stage": stage,
			"job":   job,
			"step":  step,
		}).Info("[步骤开始] ▶ 开始执行步骤")

		// 上报步骤开始日志
		stepPayload := stepMap[step]
		detailUid := ""
		if stepPayload != nil {
			detailUid = stepPayload.DetailUid // 使用步骤详情记录的自身UID
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := d.client.UploadStepLog(ctx, &client.StepLogCmd{
			JobUId:                  jobData.JobUId,
			PipelineJobStepUid:      detailUid,
			PipelineJobExecutionUid: jobData.JobExecutionUid,
			PipelineJobUid:          jobData.JobUId,
			StepName:                step,
			LogContent:              fmt.Sprintf("[%s] 开始执行步骤: %s\n", time.Now().Format("2006-01-02 15:04:05"), step),
			IsComplete:              false,
			Status:                  "RUNNING",
		})
		if err != nil {
			d.logger.WithError(err).WithFields(logrus.Fields{
				"step":      step,
				"detailUid": detailUid,
			}).Error("[步骤开始] 上传步骤日志失败")
		}
	}

	r.OnStepComplete = func(stage, job, step string, result *engine.StepResult) {
		status := "SUCCESS"
		exitCode := 0
		output := ""
		if result != nil {
			exitCode = result.ExitCode
			output = result.Stdout
			if result.ExitCode != 0 {
				status = "FAILED"
			}
		}

		d.logger.WithFields(logrus.Fields{
			"stage":    stage,
			"job":      job,
			"step":     step,
			"status":   status,
			"exitCode": exitCode,
		}).Info("[步骤完成] 步骤执行结束")

		stepPayload := stepMap[step]
		detailUid := ""
		if stepPayload != nil {
			detailUid = stepPayload.DetailUid // 使用步骤详情记录的自身UID
		}

		// 上报步骤完成日志（包含输出内容）
		logContent := output
		if logContent == "" {
			logContent = fmt.Sprintf("[%s] 步骤完成: %s, 状态: %s, 退出码: %d\n",
				time.Now().Format("2006-01-02 15:04:05"), step, status, exitCode)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := d.client.UploadStepLog(ctx, &client.StepLogCmd{
			JobUId:                  jobData.JobUId,
			PipelineJobStepUid:      detailUid,
			PipelineJobExecutionUid: jobData.JobExecutionUid,
			PipelineJobUid:          jobData.JobUId,
			StepName:                step,
			LogContent:              logContent,
			IsComplete:              true,
			ExitCode:                exitCode,
			Status:                  status,
		})
		if err != nil {
			d.logger.WithError(err).WithFields(logrus.Fields{
				"step":      step,
				"detailUid": detailUid,
				"status":    status,
			}).Error("[步骤完成] 上传步骤日志失败")
		}
	}

	r.OnJobStart = func(stage, job string) {
		d.logger.WithFields(logrus.Fields{
			"stage": stage,
			"job":   job,
		}).Info("[Job] ━━━ 开始执行任务 ━━━")
	}

	r.OnJobComplete = func(stage, job string, err error) {
		if err != nil {
			d.logger.WithError(err).WithFields(logrus.Fields{
				"stage": stage,
				"job":   job,
			}).Error("[Job] ━━━ 任务执行失败 ━━━")
		} else {
			d.logger.WithFields(logrus.Fields{
				"stage": stage,
				"job":   job,
			}).Info("[Job] ━━━ 任务执行完成 ━━━")
		}
	}
}

// reportJobResult 上报任务执行结果
func (d *Daemon) reportJobResult(jobData *client.JobData, result *runner.ExecutionResult, execErr error) {
	if jobData.JobUId == "" {
		return
	}

	status := 1 // 1=SUCCESS
	errorMsg := ""
	var duration int64

	if execErr != nil {
		status = 2 // 2=FAILURE
		errorMsg = execErr.Error()
	} else if result != nil {
		duration = result.Duration
		if !result.Success {
			status = 2
			if result.Error != nil {
				errorMsg = result.Error.Error()
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := d.client.ReportJobResult(ctx, &client.JobResult{
		JobUid:   jobData.JobExecutionUid,
		RunnerId: d.config.Uid,
		Status:   status,
		ErrorMsg: errorMsg,
		Duration: duration,
	})
	if err != nil {
		d.logger.WithError(err).WithFields(logrus.Fields{
			"component": "job_result",
			"jobUId":    jobData.JobUId,
		}).Error("上报任务执行结果失败")
	} else {
		d.logger.WithFields(logrus.Fields{
			"component": "job_result",
			"jobUId":    jobData.JobUId,
			"status":    status,
		}).Info("上报任务执行结果成功")
	}
}

// PullJobData 从服务端拉取任务数据（包含stage和job信息）
func (d *Daemon) PullJobData() (*client.JobData, error) {
	// 构造拉取任务的请求参数
	pullCmd := &client.PullJobCmd{
		RunnerUId: d.config.Uid,
		Os:        getOS(),   // 获取操作系统信息
		Arch:      getArch(), // 获取架构信息
		// 可以根据需要添加更多参数
	}

	// 调用客户端拉取任务
	jobData, err := d.client.PullJob(context.Background(), pullCmd)
	if err != nil {
		return nil, fmt.Errorf("拉取任务失败: %w", err)
	}

	// 如果没有任务，返回nil
	if jobData == nil {
		return nil, nil
	}

	return jobData, nil
}

// PullJob 从服务端拉取任务（仅返回Pipeline对象，为了兼容现有代码）
func (d *Daemon) PullJob() (*resource.Pipeline, error) {
	// 构造拉取任务的请求参数
	pullCmd := &client.PullJobCmd{
		RunnerUId: d.config.Uid,
		Os:        getOS(),   // 获取操作系统信息
		Arch:      getArch(), // 获取架构信息
		// 可以根据需要添加更多参数
	}

	// 调用客户端拉取任务
	jobData, err := d.client.PullJob(context.Background(), pullCmd)
	if err != nil {
		return nil, fmt.Errorf("拉取任务失败: %w", err)
	}

	// 如果没有任务，返回nil
	if jobData == nil {
		return nil, nil
	}

	// 解析任务数据为Pipeline对象
	pipeline, err := resource.ParseYaml([]byte(jobData.YamlConfig))
	if err != nil {
		return nil, fmt.Errorf("解析任务配置失败: %w", err)
	}

	return pipeline, nil
}

// checkForUpgrades 检查更新
func (d *Daemon) checkForUpgrades() {
	d.logger.WithFields(logrus.Fields{
		"component": "updater",
	}).Debug("检查Runner更新...")

	hasUpdate, newVersion, err := d.client.CheckUpgrade(d.ctx, d.config.Version)
	if err != nil {
		d.logger.WithError(err).WithFields(logrus.Fields{
			"component": "updater",
		}).Warn("检查更新失败")
		return
	}

	if hasUpdate {
		d.logger.WithFields(logrus.Fields{
			"component":       "updater",
			"new_version":     newVersion,
			"current_version": d.config.Version,
		}).Info("发现新版本")
		// 这里应该实现实际的更新逻辑
		// 例如：下载新版本、重启Runner等
		d.logger.WithFields(logrus.Fields{
			"component": "updater",
		}).Info("自动更新功能已启用，但更新逻辑尚未实现")
	} else {
		d.logger.WithFields(logrus.Fields{
			"component": "updater",
		}).Debug("当前已是最新版本")
	}
}

// printExecutionResult 打印执行结果
func (d *Daemon) printExecutionResult(result *runner.ExecutionResult) {
	d.logger.WithFields(logrus.Fields{
		"component": "execution_result",
		"success":   result.Success,
	}).Infof("执行结果: %s", statusStr(result.Success))
	d.logger.WithFields(logrus.Fields{
		"component": "execution_result",
		"duration":  result.Duration,
	}).Infof("总耗时: %dms", result.Duration)

	for _, stageResult := range result.StageResults {
		d.logger.WithFields(logrus.Fields{
			"component":  "execution_result",
			"stage_name": stageResult.Name,
			"stage_key":  stageResult.Key,
			"success":    stageResult.Success,
		}).Infof("阶段: %s (%s) - %s", stageResult.Name, stageResult.Key, statusStr(stageResult.Success))
		for _, jobResult := range stageResult.JobResults {
			d.logger.WithFields(logrus.Fields{
				"component": "execution_result",
				"job_name":  jobResult.Name,
				"job_key":   jobResult.Key,
				"success":   jobResult.Success,
				"duration":  jobResult.Duration,
			}).Infof("  任务: %s (%s) - %s [%dms]", jobResult.Name, jobResult.Key, statusStr(jobResult.Success), jobResult.Duration)
			for _, stepResult := range jobResult.StepResults {
				d.logger.WithFields(logrus.Fields{
					"component": "execution_result",
					"step_name": stepResult.Name,
					"step_key":  stepResult.Key,
					"success":   stepResult.Success,
					"duration":  stepResult.Duration,
				}).Infof("    步骤: %s (%s) - %s [%dms]", stepResult.Name, stepResult.Key, statusStr(stepResult.Success), stepResult.Duration)
			}
		}
	}
}

func statusStr(success bool) string {
	if success {
		return "✓ 成功"
	}
	return "✗ 失败"
}

// getOS 获取操作系统信息
func getOS() string {
	return "linux" // 示例实现，实际应该获取真实的OS信息
}

// getArch 获取架构信息
func getArch() string {
	return "amd64" // 示例实现，实际应该获取真实的架构信息
}
