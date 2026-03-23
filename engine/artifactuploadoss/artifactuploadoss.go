// Package artifactuploadoss 实现构件上传OSS引擎
package artifactuploadoss

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/DubiheStack/dubihe-runner/engine"
	"github.com/DubiheStack/dubihe-runner/resource"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/sirupsen/logrus"
)

// OSSClient OSS客户端接口
type OSSClient interface {
	InitBucket() (*oss.Bucket, error)
	UploadFile(bucket *oss.Bucket, objectKey, localFile string) error
}

// AliyunOSS 阿里云OSS实现
type AliyunOSS struct {
	Config OSSConfig
}

// OSSConfig OSS配置
type OSSConfig struct {
	AccessKeyId     string
	AccessKeySecret string
	Endpoint        string
	BucketName      string
}

// InitBucket 初始化存储桶
func (a *AliyunOSS) InitBucket() (*oss.Bucket, error) {
	client, err := oss.New(a.Config.Endpoint, a.Config.AccessKeyId, a.Config.AccessKeySecret)
	if err != nil {
		return nil, err
	}

	// 获取存储空间（Bucket）对象
	bucket, err := client.Bucket(a.Config.BucketName)
	if err != nil {
		return nil, err
	}

	return bucket, nil
}

// UploadFile 上传文件
func (a *AliyunOSS) UploadFile(bucket *oss.Bucket, objectKey, localFile string) error {
	return bucket.PutObjectFromFile(objectKey, localFile)
}

// Engine 构件上传OSS引擎
type Engine struct {
	logger *logrus.Entry
}

// New 创建构件上传OSS引擎
func New(logger *logrus.Entry) *Engine {
	if logger == nil {
		logger = logrus.WithField("engine", "artifactuploadoss")
	}
	return &Engine{
		logger: logger,
	}
}

// Name 返回引擎名称
func (e *Engine) Name() string {
	return "artifactuploadoss"
}

// Ping 检查引擎状态
func (e *Engine) Ping(ctx context.Context) error {
	// 检查阿里云OSS SDK是否可用
	return nil
}

// Setup 准备执行环境
func (e *Engine) Setup(ctx context.Context, job *resource.Job, opts *engine.Options) error {
	e.logger.WithField("job", job.Key).Info("Setting up artifact upload oss environment")
	return nil
}

// Execute 执行构件上传OSS步骤
func (e *Engine) Execute(ctx context.Context, step *resource.Step, opts *engine.Options) (*engine.StepResult, error) {
	e.logger.WithField("step", step.Key).WithField("name", step.Name).Info("Executing artifact upload oss step")

	startTime := time.Now()
	result := &engine.StepResult{}

	// 检查步骤类型是否为构件上传OSS
	if step.Step != resource.StepTypeArtifactUploadOss {
		return result, fmt.Errorf("step type is not artifactUploadOss: %s", step.Step)
	}

	// 获取构件上传OSS步骤参数
	uploadStep, ok := step.With.(*resource.ArtifactUploadOssStep)
	if !ok {
		return result, fmt.Errorf("invalid artifact upload oss step configuration")
	}

	// 执行上传逻辑
	if err := e.uploadArtifact(ctx, opts, uploadStep); err != nil {
		result.ExitCode = 1
		if !step.ContinueOnError {
			return result, fmt.Errorf("artifact upload oss step %s failed: %w", step.Key, err)
		}
		e.logger.WithField("step", step.Key).WithField("exitCode", result.ExitCode).Warn("Artifact upload oss step failed but continuing")
	} else {
		e.logger.WithField("step", step.Key).Info("Artifact upload oss step completed successfully")
	}

	result.Duration = time.Since(startTime).Milliseconds()
	return result, nil
}

// uploadArtifact 上传构件
func (e *Engine) uploadArtifact(ctx context.Context, opts *engine.Options, config *resource.ArtifactUploadOssStep) error {
	e.logger.Info("Starting artifact upload to OSS process")

	// 确定工作目录
	workDir := opts.WorkDir
	if workDir == "" {
		workDir = "."
	}

	// 构建可能的文件路径
	var fullCodePath string
	possiblePaths := []string{
		filepath.Join(workDir, config.CodePath), // 当前工作目录下的路径
		config.CodePath,                         // 绝对路径或相对于当前工作目录的路径
		filepath.Join(".", config.CodePath),     // 相对于当前目录的路径
	}

	// 添加常见的源码目录路径
	sourcesBasePaths := []string{
		filepath.Join(workDir, "..", "..", "sources", "repo_1"),
		filepath.Join(workDir, "..", "..", "sources"),
		filepath.Join(workDir, "sources", "repo_1"),
		filepath.Join(".", "sources", "repo_1"),
		filepath.Join(workDir, "sources"),
		filepath.Join(".", "sources"),
	}

	for _, basePath := range sourcesBasePaths {
		possiblePaths = append(possiblePaths, filepath.Join(basePath, config.CodePath))
	}

	// 尝试找到文件
	found := false
	for _, path := range possiblePaths {
		if path != "" {
			e.logger.Debugf("Checking for file at: %s", path)
			if _, err := os.Stat(path); err == nil {
				fullCodePath = path
				found = true
				e.logger.Infof("File found at: %s", path)
				break
			}
		}
	}

	if !found {
		// 如果都没有找到，列出所有尝试过的路径用于调试
		e.logger.Warnf("Could not find code file. Attempted paths: %v", possiblePaths)
		return fmt.Errorf("can not find code file: %s", config.CodePath)
	}

	// 检查文件是否存在
	if _, err := os.Stat(fullCodePath); os.IsNotExist(err) {
		return fmt.Errorf("can not find code file: %s", fullCodePath)
	}
	e.logger.Infof("File exists: %s", fullCodePath)

	// 创建OSS配置
	ossCfg := OSSConfig{
		AccessKeyId:     config.AccessKeyId,
		AccessKeySecret: config.AccessKeySecret,
		Endpoint:        config.Endpoint,
		BucketName:      config.BucketName,
	}

	e.logger.Debugf("OSS Config: %+v", ossCfg)

	var ossInstance OSSClient

	// 根据OSS类型创建实例
	switch config.OssType {
	case "aliyun":
		ossInstance = &AliyunOSS{ossCfg}
	case "tencent":
		// TODO: 添加腾讯云OSS支持
		return fmt.Errorf("tencent oss type not implemented yet")
	default:
		return fmt.Errorf("unsupported oss type: %s", config.OssType)
	}

	// 构建对象键
	objectKey := fmt.Sprintf("%s/%s/%s/%s", config.AppName, config.Environment, config.PackageVersion, filepath.Base(config.CodePath))

	e.logger.Infof("Uploading file %s to OSS with key %s", fullCodePath, objectKey)

	// 初始化存储桶
	bucket, err := ossInstance.InitBucket()
	if err != nil {
		e.logger.Errorf("Failed to init OSS bucket: %v", err)
		return fmt.Errorf("init oss client or bucket failed: %w", err)
	}
	e.logger.Info("Successfully initialized OSS bucket")

	// 上传文件
	if err := ossInstance.UploadFile(bucket, objectKey, fullCodePath); err != nil {
		e.logger.Errorf("Failed to upload file to OSS: %v", err)
		return fmt.Errorf("upload file to oss failed: %w", err)
	}

	// 添加成功上传的日志
	e.logger.Infof("SUCCESS: File %s uploaded to OSS successfully with object key: %s", fullCodePath, objectKey)
	e.logger.Infof("OSS Bucket: %s", config.BucketName)
	e.logger.Infof("OSS Endpoint: %s", config.Endpoint)

	// 构建OSS访问路径
	ossPath := fmt.Sprintf("%s/%s", config.BathUrl, objectKey)

	// 计算文件MD5
	md5sum, err := e.genMd5(fullCodePath)
	if err != nil {
		e.logger.Errorf("Failed to generate MD5: %v", err)
		return fmt.Errorf("generate md5sum failed: %w", err)
	}
	e.logger.Infof("File MD5: %s", md5sum)

	// 构建回调参数
	params := url.Values{}
	params.Add("noahPublishId", strconv.FormatInt(config.NoahPublishBillId, 10))
	params.Add("appName", config.AppName)
	params.Add("deployOperator", config.Operator)
	params.Add("packageVersion", config.PackageVersion)
	params.Add("env", config.Environment)
	params.Add("url", ossPath)
	params.Add("md5", md5sum)

	e.logger.Infof("Sending callback to %s with params: %s", config.CallBackUrl, params.Encode())

	// 发送回调请求
	if err := e.sendCallback(config.CallBackUrl, params); err != nil {
		e.logger.Errorf("Failed to send callback: %v", err)
		return fmt.Errorf("send callback failed: %w", err)
	}
	e.logger.Info("Callback sent successfully")

	e.logger.Info("Artifact upload to OSS process completed successfully")
	return nil
}

// genMd5 计算文件MD5
func (e *Engine) genMd5(filePath string) (string, error) {
	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 创建一个MD5哈希对象
	hash := md5.New()

	// 将文件内容拷贝到哈希对象中
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	// 计算MD5校验码
	checksum := hash.Sum(nil)

	// 将校验码转换为字符串表示
	md5sum := hex.EncodeToString(checksum)

	return md5sum, nil
}

// sendCallback 发送回调请求
func (e *Engine) sendCallback(callbackUrl string, params url.Values) error {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.PostForm(callbackUrl, params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("callback request failed with status: %d", resp.StatusCode)
	}

	return nil
}

// Destroy 清理执行环境
func (e *Engine) Destroy(ctx context.Context) error {
	e.logger.Info("Destroying artifact upload oss environment")
	return nil
}
