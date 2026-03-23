// Package client 提供与服务端通信的HTTP客户端
package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/DubiheStack/dubihe-runner/license"
	"github.com/DubiheStack/dubihe-runner/monitor"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const (
	endpointRegister            = "/api/runner/register"
	endpointHeartbeat           = "/api/runner/heartBeat"
	endpointCredential          = "/api/runner/credential"
	endpointReportResourceUsage = "/api/runner/reportResourceUsage"
	endpointPullJob             = "/api/runner/pullJob"         // 新增拉取任务的端点
	endpointReportJobResult     = "/api/runner/reportJobResult" // 新增上报任务结果的端点
	endpointUploadStepLog       = "/api/runner/stepLog"         // 上传步骤日志的端点
	defaultConfigFile           = "config.yaml"

	// DefaultHeartbeatInterval 默认心跳间隔
	DefaultHeartbeatInterval = 30 * time.Second
	// DefaultScanInterval 默认扫描拉取任务间隔
	DefaultScanInterval = 10 * time.Second
)

// Config Runner配置文件结构
type Config struct {
	Uid               string     `yaml:"uid"`
	Url               string     `yaml:"url"`
	Token             string     `yaml:"token"`
	Tenant            string     `yaml:"tenant"`
	HostUid           string     `yaml:"hostUid"`
	Type              string     `yaml:"type"`
	Version           string     `yaml:"version"`
	HeartbeatInterval int        `yaml:"heartbeatInterval"` // 心跳时间，Runner与服务端发起心跳的时间间隔
	ScanInterval      int        `yaml:"scanInterval"`      // runner扫描拉取Job的间隔时间
	Concurrency       int        `yaml:"concurrency"`       // 最大并发任务数，默认1
	AutoUpgrade       bool       `yaml:"autoUpgrade"`       // 是否开启自动更新
	UpgradeInterval   int        `yaml:"upgradeInterval"`   // 更新检查间隔(秒)，默认300秒
	WorkDir           string     `yaml:"workDir"`           // 工作空间目录，默认为当前目录
	LogConfig         *LogConfig `yaml:"logConfig"`         // 日志配置
	LogFile           string     `yaml:"logFile"`           // 日志文件路径
	DevMode           bool       `yaml:"devMode"`           // 开发模式
	LicensePath       string     `yaml:"licensePath"`       // License文件路径
}

// LogConfig 日志配置结构
type LogConfig struct {
	LogType          string `yaml:"logType"`          // 日志类型
	BatchSubmitLines int    `yaml:"batchSubmitLines"` // 批量提交行数
	RefreshDuration  int    `yaml:"refreshDuration"`  // 刷新间隔(毫秒)
	FlushDuration    int    `yaml:"flushDuration"`    // 刷新持续时间(毫秒)
	LogFile          string `yaml:"logFile"`          // 日志文件路径
}

// PullJobCmd 拉取任务命令
type PullJobCmd struct {
	RunnerUId string            `json:"runnerUId"`
	Os        string            `json:"os"`
	Arch      string            `json:"arch"`
	Variant   string            `json:"variant"`
	Kernel    string            `json:"kernel"`
	HostIp    string            `json:"hostIp"`
	Labels    map[string]string `json:"labels"`
}

// JobData 任务数据（完整结构，对应Java端JobToRunnerCO）
type JobData struct {
	JobUId            string        `json:"jobUId"`
	JobExecutionUid   string        `json:"jobExecutionUid"`
	PipelineUId       string        `json:"pipelineUId"`
	StageExecutionUid string        `json:"stageExecutionUid"`
	StageName         string        `json:"stageName"`
	JobName           string        `json:"jobName"`
	YamlConfig        string        `json:"yamlConfig"`
	Payload           []StepPayload `json:"payload"`
	Requirements      interface{}   `json:"requirements"`
}

// StepPayload 步骤负载信息
type StepPayload struct {
	StepUid   string `json:"stepUid"`
	StepName  string `json:"stepName"`
	DetailUid string `json:"detailUid"`
	Sequence  int    `json:"sequence"`
}

// StepLogCmd 步骤日志上传命令
type StepLogCmd struct {
	JobUId                  string `json:"jobUId"`
	PipelineExecutionUid    string `json:"pipelineExecutionUid"`
	PipelineJobStepUid      string `json:"pipelineJobStepUid"`
	PipelineJobExecutionUid string `json:"pipelineJobExecutionUid"`
	PipelineJobUid          string `json:"pipelineJobUid"`
	StepName                string `json:"stepName"`
	LogContent              string `json:"logContent"`
	IsComplete              bool   `json:"isComplete"`
	ExitCode                int    `json:"exitCode"`
	Status                  string `json:"status"`
}

// JobResult 任务执行结果
type JobResult struct {
	JobUid   string `json:"jobUid"`
	RunnerId string `json:"runnerId"`
	Status   int    `json:"status"`
	ErrorMsg string `json:"errorMsg"`
	Duration int64  `json:"duration"`
	Output   string `json:"output"`
}

// LoadConfig 从文件加载配置
func LoadConfig(filePath string) (*Config, error) {
	if filePath == "" {
		filePath = defaultConfigFile
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &cfg, nil
}

// ToRunnerMachine 将配置转换为RunnerMachine
func (c *Config) ToRunnerMachine() *RunnerMachine {
	return &RunnerMachine{
		HostUid: c.HostUid,
		Tenant:  c.Tenant,
		Token:   c.Token,
		Type:    c.Type,
		Uid:     c.Uid,
		Version: c.Version,
	}
}

// GetLogConfig 获取日志配置，如果未设置则返回默认配置
func (c *Config) GetLogConfig() *LogConfig {
	if c.LogConfig == nil {
		return &LogConfig{
			LogType:          "default",
			BatchSubmitLines: 100,
			RefreshDuration:  1000,
			FlushDuration:    5000,
			LogFile:          "./workspace/log/dubihe-runner.log",
		}
	}
	// 如果LogConfig中没有设置LogFile，使用默认值
	if c.LogConfig.LogFile == "" {
		c.LogConfig.LogFile = "./workspace/log/dubihe-runner.log"
	}
	return c.LogConfig
}

// GetHeartbeatInterval 获取心跳间隔
func (c *Config) GetHeartbeatInterval() time.Duration {
	if c.HeartbeatInterval > 0 {
		return time.Duration(c.HeartbeatInterval) * time.Second
	}
	return DefaultHeartbeatInterval
}

// GetScanInterval 获取扫描间隔
func (c *Config) GetScanInterval() time.Duration {
	if c.ScanInterval > 0 {
		return time.Duration(c.ScanInterval) * time.Second
	}
	return DefaultScanInterval // 默认使用独立的扫描间隔时间
}

// GetConcurrency 获取并发任务数
func (c *Config) GetConcurrency() int {
	if c.Concurrency > 0 {
		return c.Concurrency
	}
	return 1 // 默认并发数为1
}

// GetUpgradeInterval 获取更新检查间隔
func (c *Config) GetUpgradeInterval() time.Duration {
	if c.UpgradeInterval > 0 {
		return time.Duration(c.UpgradeInterval) * time.Second
	}
	return 300 * time.Second // 默认5分钟检查一次更新
}

// GetWorkDir 获取工作目录
func (c *Config) GetWorkDir() string {
	if c.WorkDir != "" {
		return c.WorkDir
	}
	// 默认返回当前目录
	if dir, err := os.Getwd(); err == nil {
		return dir
	}
	return "."
}

// GetLicensePath 获取License文件路径
func (c *Config) GetLicensePath() string {
	if c.LicensePath != "" {
		return c.LicensePath
	}
	// 默认返回当前目录下的license文件
	return "./license/license"
}

// Response 服务端响应结构
type Response struct {
	Code       int         `json:"code"`
	MsgCode    string      `json:"msgCode"`
	MsgContent string      `json:"msgContent"`
	Data       interface{} `json:"data"`
}

// RunnerMachine Runner机器信息
type RunnerMachine struct {
	HostUid string `json:"hostUid"`
	Tenant  string `json:"tenant"`
	Token   string `json:"token"`
	Type    string `json:"type"`
	Uid     string `json:"uid"`
	Version string `json:"version"`
}

// Credential 凭证信息
type Credential struct {
	Name       string `json:"name"`
	Url        string `json:"url"`
	Type       string `json:"type"` // token, password, ssh
	Uid        string `json:"uid"`
	Token      string `json:"token"`
	UserName   string `json:"userName"`
	Password   string `json:"password"`
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
}

// CredentialResponse 凭证响应
type CredentialResponse struct {
	Code       int         `json:"code"`
	MsgCode    string      `json:"msgCode"`
	MsgContent string      `json:"msgContent"`
	Data       *Credential `json:"data"`
}

// ResourceUsage 资源使用情况
type ResourceUsage struct {
	CPU    string `json:"cpu"`     // CPU 使用率
	Mem    string `json:"mem"`     // 内存使用量(MB)
	Disk   string `json:"disk"`    // 磁盘使用量(MB)
	HostID string `json:"hostUid"` // 主机ID
}

// defaultClient 默认HTTP客户端
var defaultClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// Client Runner客户端
type Client struct {
	httpClient *http.Client
	endpoint   string
	token      string
	skipVerify bool
	logger     *logrus.Entry
}

// New 创建新的客户端
func New(endpoint, token string, skipVerify bool) *Client {
	c := &Client{
		endpoint:   endpoint,
		token:      token,
		skipVerify: skipVerify,
		logger:     logrus.WithField("component", "client"),
	}

	if skipVerify {
		c.httpClient = &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
	}

	return c
}

// Register 向服务端注册Runner
func (c *Client) Register(ctx context.Context, info *RunnerMachine) error {
	c.logger.WithFields(logrus.Fields{
		"hostUid":   info.HostUid,
		"tenant":    info.Tenant,
		"type":      info.Type,
		"component": "registration",
	}).Info("正在向服务端注册Runner...")

	dst := new(Response)
	err := c.doRequest(ctx, endpointRegister, http.MethodPost, info, dst)
	if err != nil {
		c.logger.WithError(err).WithFields(logrus.Fields{
			"component": "registration",
		}).Error("Runner注册失败")
		return err
	}

	if dst.Code != 0 && dst.Code != 200 {
		err := fmt.Errorf("注册失败: %s (code: %d)", dst.MsgContent, dst.Code)
		c.logger.WithError(err).WithFields(logrus.Fields{
			"component": "registration",
		}).Error("Runner注册失败")
		return err
	}

	c.logger.WithFields(logrus.Fields{
		"component": "registration",
	}).Info("Runner注册成功")
	return nil
}

// Heartbeat 发送心跳
func (c *Client) Heartbeat(ctx context.Context, info *RunnerMachine) error {
	dst := new(Response)
	err := c.doRequest(ctx, endpointHeartbeat, http.MethodPost, info, dst)
	if err != nil {
		c.logger.WithError(err).WithFields(logrus.Fields{
			"component": "heartbeat",
		}).Debug("心跳发送失败")
		return err
	}

	if dst.Code != 0 && dst.Code != 200 {
		err := fmt.Errorf("心跳失败: %s (code: %d)", dst.MsgContent, dst.Code)
		c.logger.WithError(err).WithFields(logrus.Fields{
			"component": "heartbeat",
		}).Debug("心跳发送失败")
		return err
	}

	c.logger.WithFields(logrus.Fields{
		"component": "heartbeat",
	}).Debug("心跳发送成功")
	return nil
}

// ReportResourceUsage 上报资源使用情况
func (c *Client) ReportResourceUsage(ctx context.Context, usage *ResourceUsage) error {
	c.logger.WithFields(logrus.Fields{
		"cpu":       usage.CPU,
		"memory":    usage.Mem,
		"disk":      usage.Disk,
		"hostId":    usage.HostID,
		"component": "resource",
	}).Debug("正在上报资源使用情况...")

	dst := new(Response)
	err := c.doRequest(ctx, endpointReportResourceUsage, http.MethodPost, usage, dst)
	if err != nil {
		c.logger.WithError(err).WithFields(logrus.Fields{
			"component": "resource",
		}).Debug("资源使用情况上报失败")
		return err
	}

	if dst.Code != 0 && dst.Code != 200 {
		err := fmt.Errorf("资源使用情况上报失败: %s (code: %d)", dst.MsgContent, dst.Code)
		c.logger.WithError(err).WithFields(logrus.Fields{
			"component": "resource",
		}).Debug("资源使用情况上报失败")
		return err
	}

	c.logger.WithFields(logrus.Fields{
		"component": "resource",
	}).Debug("资源使用情况上报成功")
	return nil
}

// GetCredential 获取凭证信息
func (c *Client) GetCredential(ctx context.Context, uid string) (*Credential, error) {
	c.logger.WithFields(logrus.Fields{
		"uid":       uid,
		"component": "credential",
	}).Debug("正在获取凭证信息...")

	path := fmt.Sprintf("%s?uid=%s", endpointCredential, uid)
	dst := new(CredentialResponse)
	err := c.doRequest(ctx, path, http.MethodGet, nil, dst)
	if err != nil {
		c.logger.WithError(err).WithFields(logrus.Fields{
			"component": "credential",
		}).Error("获取凭证失败")
		return nil, err
	}

	if dst.Code != 0 && dst.Code != 200 {
		err := fmt.Errorf("获取凭证失败: %s (code: %d)", dst.MsgContent, dst.Code)
		c.logger.WithError(err).WithFields(logrus.Fields{
			"component": "credential",
		}).Error("获取凭证失败")
		return nil, err
	}

	if dst.Data == nil {
		err := fmt.Errorf("凭证不存在: %s", uid)
		c.logger.WithError(err).WithFields(logrus.Fields{
			"component": "credential",
		}).Error("凭证不存在")
		return nil, err
	}

	c.logger.WithFields(logrus.Fields{
		"name":      dst.Data.Name,
		"type":      dst.Data.Type,
		"component": "credential",
	}).Debug("获取凭证成功")

	return dst.Data, nil
}

// PullJob 拉取任务
func (c *Client) PullJob(ctx context.Context, cmd *PullJobCmd) (*JobData, error) {
	c.logger.WithFields(logrus.Fields{
		"component": "job",
	}).Debug("正在拉取任务...")

	dst := new(Response)
	err := c.doRequest(ctx, endpointPullJob, http.MethodPost, cmd, dst)
	if err != nil {
		c.logger.WithError(err).WithFields(logrus.Fields{
			"component": "job",
		}).Error("拉取任务失败")
		return nil, err
	}

	if dst.Code != 0 && dst.Code != 200 {
		err := fmt.Errorf("拉取任务失败: %s (code: %d)", dst.MsgContent, dst.Code)
		c.logger.WithError(err).WithFields(logrus.Fields{
			"component": "job",
		}).Error("拉取任务失败")
		return nil, err
	}

	if dst.Data == nil {
		// 没有任务可拉取
		c.logger.WithFields(logrus.Fields{
			"component": "job",
		}).Debug("没有任务可拉取")
		return nil, nil
	}

	// 将数据转换为JobData
	dataBytes, err := json.Marshal(dst.Data)
	if err != nil {
		c.logger.WithError(err).WithFields(logrus.Fields{
			"component": "job",
		}).Error("解析任务数据失败")
		return nil, err
	}

	var jobData JobData
	if err := json.Unmarshal(dataBytes, &jobData); err != nil {
		c.logger.WithError(err).WithFields(logrus.Fields{
			"component": "job",
		}).Error("解析任务数据失败")
		return nil, err
	}

	c.logger.WithFields(logrus.Fields{
		"component": "job",
	}).Debug("拉取任务成功")
	return &jobData, nil
}

// ReportJobResult 上报任务执行结果
func (c *Client) ReportJobResult(ctx context.Context, result *JobResult) error {
	c.logger.WithFields(logrus.Fields{
		"jobUid":    result.JobUid,
		"status":    result.Status,
		"component": "job",
	}).Debug("正在上报任务执行结果...")

	dst := new(Response)
	err := c.doRequest(ctx, endpointReportJobResult, http.MethodPost, result, dst)
	if err != nil {
		c.logger.WithError(err).WithFields(logrus.Fields{
			"component": "job",
		}).Error("上报任务执行结果失败")
		return err
	}

	if dst.Code != 0 && dst.Code != 200 {
		err := fmt.Errorf("上报任务执行结果失败: %s (code: %d)", dst.MsgContent, dst.Code)
		c.logger.WithError(err).WithFields(logrus.Fields{
			"component": "job",
		}).Error("上报任务执行结果失败")
		return err
	}

	c.logger.WithFields(logrus.Fields{
		"component": "job",
	}).Debug("上报任务执行结果成功")
	return nil
}

// UploadStepLog 上传步骤执行日志
func (c *Client) UploadStepLog(ctx context.Context, cmd *StepLogCmd) error {
	c.logger.WithFields(logrus.Fields{
		"jobUId":     cmd.JobUId,
		"stepName":   cmd.StepName,
		"isComplete": cmd.IsComplete,
		"component":  "step_log",
	}).Debug("正在上传步骤日志...")

	dst := new(Response)
	err := c.doRequest(ctx, endpointUploadStepLog, http.MethodPost, cmd, dst)
	if err != nil {
		c.logger.WithError(err).WithFields(logrus.Fields{
			"component": "step_log",
		}).Error("上传步骤日志失败")
		return err
	}

	if dst.Code != 0 && dst.Code != 200 {
		err := fmt.Errorf("上传步骤日志失败: %s (code: %d)", dst.MsgContent, dst.Code)
		c.logger.WithError(err).WithFields(logrus.Fields{
			"component": "step_log",
		}).Error("上传步骤日志失败")
		return err
	}

	c.logger.WithFields(logrus.Fields{
		"component": "step_log",
	}).Debug("上传步骤日志成功")
	return nil
}

// CheckUpgrade 检查是否有新版本
func (c *Client) CheckUpgrade(ctx context.Context, currentVersion string) (bool, string, error) {
	// 这里应该实现检查更新的逻辑
	// 暂时返回false表示没有更新
	return false, currentVersion, nil
}

// doRequest 执行HTTP请求
func (c *Client) doRequest(ctx context.Context, path, method string, in, out interface{}) error {
	var buf bytes.Buffer
	if in != nil {
		if err := json.NewEncoder(&buf).Encode(in); err != nil {
			return fmt.Errorf("编码请求体失败: %w", err)
		}
	}

	url := c.endpoint + path
	req, err := http.NewRequestWithContext(ctx, method, url, &buf)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Dubihe-Token", c.token)

	client := c.httpClient
	if client == nil {
		client = defaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer func() {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode > 299 {
		if len(body) > 0 {
			return errors.New(string(body))
		}
		return errors.New(http.StatusText(resp.StatusCode))
	}

	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("解析响应失败: %w", err)
		}
	}

	return nil
}

// GetHostUID 获取主机唯一标识
func GetHostUID() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// HeartbeatManager 心跳管理器
type HeartbeatManager struct {
	client   *Client
	info     *RunnerMachine
	interval time.Duration
	logger   *logrus.Entry

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewHeartbeatManager 创建心跳管理器
func NewHeartbeatManager(client *Client, info *RunnerMachine, interval time.Duration) *HeartbeatManager {
	if interval <= 0 {
		interval = DefaultHeartbeatInterval
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 创建带输出设置的日志记录器
	logger := logrus.WithField("component", "heartbeat")

	return &HeartbeatManager{
		client:   client,
		info:     info,
		interval: interval,
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start 启动心跳任务
func (h *HeartbeatManager) Start() {
	h.wg.Add(1)
	go h.run()
	h.logger.WithField("interval", h.interval).Info("[Heartbeat] 心跳任务已启动")
}

// Stop 停止心跳任务
func (h *HeartbeatManager) Stop() {
	h.cancel()
	h.wg.Wait()
	h.logger.Info("[Heartbeat] 心跳任务已停止")
}

// run 运行心跳循环
func (h *HeartbeatManager) run() {
	defer h.wg.Done()

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	// 首次立即发送心跳
	h.sendHeartbeat()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.sendHeartbeat()
			// 同时上报资源使用情况
			h.reportResourceUsage()
		}
	}
}

// sendHeartbeat 发送心跳
func (h *HeartbeatManager) sendHeartbeat() {
	ctx, cancel := context.WithTimeout(h.ctx, 10*time.Second)
	defer cancel()

	// 在发送心跳前检查许可证
	if !h.checkLicense() {
		h.logger.Error("[License] License check failed, stopping heartbeat")
		return
	}

	err := h.client.Heartbeat(ctx, h.info)
	if err != nil {
		h.logger.WithError(err).Warn("[Heartbeat] 心跳发送失败，尝试重新注册...")
		// 心跳失败时尝试重新注册
		if regErr := h.client.Register(ctx, h.info); regErr != nil {
			h.logger.WithError(regErr).Error("[Heartbeat] 重新注册失败")
		} else {
			h.logger.Info("[Heartbeat] 重新注册成功")
		}
	} else {
		h.logger.Info("[Heartbeat] 心跳发送成功")
	}

	// 记录心跳执行，无论成功与否
	h.logger.Info("[Heartbeat] 心跳周期性检查执行")
}

// checkLicense 检查许可证
func (h *HeartbeatManager) checkLicense() bool {
	// 获取许可证检查器实例
	checker := license.GetInstance()
	licenseInfo := checker.GetLicenseInfo()

	// 如果没有许可证信息，允许继续运行
	if licenseInfo == nil {
		h.logger.Info("[License] No license found, continuing heartbeat")
		return true
	}

	// 检查是否有必要的许可证信息
	if licenseInfo.GetLimitDays() == "" && licenseInfo.GetStartDate() == "" {
		h.logger.Info("[License] License has no expiration, continuing heartbeat")
		return true
	}

	// 检查许可证是否过期
	if license.IsExpired() {
		h.logger.Error("license已过期，请联系授权方进行license更新。")
		return false
	}

	// 检查是否为同一台服务器
	if !license.IsSameServerAuto() {
		h.logger.Error("[License] License is not valid for this server")
		return false
	}

	h.logger.Info("[License] License check passed")
	return true
}

// reportResourceUsage 上报资源使用情况
func (h *HeartbeatManager) reportResourceUsage() {
	ctx, cancel := context.WithTimeout(h.ctx, 10*time.Second)
	defer cancel()

	usage, err := monitor.Collect(h.info.HostUid)
	if err != nil {
		h.logger.WithError(err).Warn("[Resource] 收集资源使用情况失败")
		// 即使收集失败也记录尝试
		h.logger.Info("[Resource] 资源使用情况周期性检查执行")
		return
	}

	err = h.client.ReportResourceUsage(ctx, &ResourceUsage{
		CPU:    usage.CPU,
		Mem:    usage.Mem,
		Disk:   usage.Disk,
		HostID: usage.HostID,
	})
	if err != nil {
		h.logger.WithError(err).Warn("[Resource] 资源使用情况上报失败")
		// 即使上报失败也记录收集到的数据
		h.logger.WithFields(logrus.Fields{
			"cpu":    usage.CPU,
			"memory": usage.Mem,
			"disk":   usage.Disk,
		}).Info("[Resource] 资源使用情况收集成功但上报失败")
	} else {
		h.logger.WithFields(logrus.Fields{
			"cpu":    usage.CPU,
			"memory": usage.Mem,
			"disk":   usage.Disk,
		}).Info("[Resource] 资源使用情况上报成功")
	}

	// 记录资源监控执行
	h.logger.Info("[Resource] 资源监控周期性检查执行")
}
