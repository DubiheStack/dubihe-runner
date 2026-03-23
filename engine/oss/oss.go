package oss

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	oss_sdk "github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/sirupsen/logrus"

	"github.com/DubiheStack/dubihe-runner/engine"
	"github.com/DubiheStack/dubihe-runner/resource"
	oss_resource "github.com/DubiheStack/dubihe-runner/resource/oss"
)

// OSSUploader 实现OSS上传功能
type OSSUploader struct {
	logger *logrus.Entry
}

// New 创建新的OSS上传器
func New(logger *logrus.Entry) *OSSUploader {
	if logger == nil {
		logger = logrus.NewEntry(logrus.StandardLogger())
	}
	return &OSSUploader{
		logger: logger,
	}
}

// Upload 执行OSS上传操作
func (o *OSSUploader) Upload(ctx context.Context, step *resource.Step, opts *engine.Options) (*engine.StepResult, error) {
	o.logger.WithField("step", step.Key).Info("开始执行OSS上传步骤")

	result := &engine.StepResult{
		ExitCode: 0,
		Stdout:   "",
		Stderr:   "",
	}

	// 获取OSS上传参数
	ossParams, err := o.parseOSSParams(step)
	if err != nil {
		result.ExitCode = 1
		result.Stderr = err.Error()
		return result, fmt.Errorf("解析OSS参数失败: %w", err)
	}

	// 获取服务连接凭证（这里需要从配置或环境变量获取）
	accessKeyID := os.Getenv(fmt.Sprintf("SERVICE_CONNECTION_%s_ACCESS_KEY", strings.ToUpper(ossParams.ServiceConnection)))
	accessKeySecret := os.Getenv(fmt.Sprintf("SERVICE_CONNECTION_%s_SECRET_KEY", strings.ToUpper(ossParams.ServiceConnection)))
	if accessKeyID == "" || accessKeySecret == "" {
		// 尝試從環境變量獲取通用的OSS憑證
		accessKeyID = os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_ID")
		accessKeySecret = os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET")
		if accessKeyID == "" || accessKeySecret == "" {
			result.ExitCode = 1
			result.Stderr = "未找到OSS服务连接凭证，请配置SERVICE_CONNECTION_*_ACCESS_KEY和SERVICE_CONNECTION_*_SECRET_KEY环境变量"
			return result, fmt.Errorf("未找到OSS服务连接凭证")
		}
	}

	// 创建OSS客户端
	client, err := oss_sdk.New(fmt.Sprintf("https://oss-%s.aliyuncs.com", ossParams.Region), accessKeyID, accessKeySecret)
	if err != nil {
		result.ExitCode = 1
		result.Stderr = err.Error()
		return result, fmt.Errorf("创建OSS客户端失败: %w", err)
	}
	o.logger.Info("成功创建OSS客户端")

	// 确定工作目录
	workDir := opts.WorkDir
	if workDir == "" {
		workDir = "."
	}

	// 处理源文件路径
	sourcePath := filepath.Join(workDir, ossParams.SourceFilePath)
	targetPath := ossParams.TargetFilePath

	// 检查源文件是否存在
	fileInfo, err := os.Stat(sourcePath)
	if err != nil {
		result.ExitCode = 1
		result.Stderr = fmt.Sprintf("源文件路径不存在: %s", sourcePath)
		return result, fmt.Errorf("源文件路径不存在: %s", sourcePath)
	}

	o.logger.Infof("开始上传文件/目录: %s -> OSS bucket: %s, target path: %s", sourcePath, ossParams.Bucket, targetPath)

	// 输出文件/目录信息
	if fileInfo.IsDir() {
		o.logger.Infof("检测到目录上传模式，目录路径: %s", sourcePath)
		entries, err := os.ReadDir(sourcePath)
		if err != nil {
			o.logger.Errorf("无法读取目录: %v", err)
		} else {
			o.logger.Infof("目录包含 %d 个项目", len(entries))
			for _, entry := range entries {
				o.logger.Infof("- %s (%s)", entry.Name(), map[bool]string{true: "dir", false: "file"}[entry.IsDir()])
			}
		}
	} else {
		o.logger.Infof("检测到文件上传模式，文件大小: %d bytes", fileInfo.Size())
	}

	// 如果是目录，需要递归上传
	if fileInfo.IsDir() {
		o.logger.Info("检测到目录上传模式")
		err = o.uploadDirectory(client, ossParams.Bucket, sourcePath, targetPath, ossParams.Metas)
	} else {
		o.logger.Info("检测到文件上传模式")
		err = o.uploadFile(client, ossParams.Bucket, sourcePath, targetPath, ossParams.Metas)
	}

	if err != nil {
		result.ExitCode = 1
		result.Stderr = err.Error()
		return result, fmt.Errorf("上传失败: %w", err)
	}

	result.Stdout = fmt.Sprintf("OSS上传成功: %s -> oss://%s/%s", sourcePath, ossParams.Bucket, targetPath)
	o.logger.WithField("step", step.Key).Infof("OSS上传完成: %s -> oss://%s/%s", sourcePath, ossParams.Bucket, targetPath)

	return result, nil
}

// parseOSSParams 解析OSS上传参数
func (o *OSSUploader) parseOSSParams(step *resource.Step) (*oss_resource.OSSUploadStep, error) {
	if step.With == nil {
		return nil, fmt.Errorf("OSSUpload步骤缺少with参数")
	}

	ossStep, err := oss_resource.ParseOSSUploadStep(step.With)
	if err != nil || ossStep == nil {
		return nil, fmt.Errorf("OSSUpload步骤参数解析失败: %v", err)
	}

	if ossStep.ServiceConnection == "" {
		return nil, fmt.Errorf("serviceConnection参数不能为空")
	}

	if ossStep.Region == "" {
		return nil, fmt.Errorf("region参数不能为空")
	}

	if ossStep.Bucket == "" {
		return nil, fmt.Errorf("bucket参数不能为空")
	}

	if ossStep.SourceFilePath == "" {
		return nil, fmt.Errorf("sourceFilePath参数不能为空")
	}

	if ossStep.TargetFilePath == "" {
		ossStep.TargetFilePath = ossStep.SourceFilePath
	}

	return ossStep, nil
}

// uploadFile 上传单个文件到OSS
func (o *OSSUploader) uploadFile(client *oss_sdk.Client, bucketName, localPath, targetPath string, metas []*resource.KeyVal) error {
	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return fmt.Errorf("获取OSS存储桶失败: %w", err)
	}

	// 确保目标路径格式正确
	objectKey := strings.TrimPrefix(targetPath, "/")
	if !strings.HasSuffix(objectKey, filepath.Base(localPath)) {
		objectKey = filepath.Join(objectKey, filepath.Base(localPath))
	}

	// 构建OSS对象元数据
	options := []oss_sdk.Option{}

	// 添加用户指定的元数据
	for _, meta := range metas {
		if meta.Key == "html" {
			// 特殊处理html元数据，它包含完整的头部信息如"Cache-Control:no-cache"
			parts := strings.SplitN(meta.Value, ":", 2)
			if len(parts) == 2 {
				headerKey := strings.TrimSpace(parts[0])
				headerValue := strings.TrimSpace(parts[1])
				options = append(options, oss_sdk.SetHeader(headerKey, headerValue))
			}
		} else {
			options = append(options, oss_sdk.Meta(meta.Key, meta.Value))
		}
	}

	// 上传文件
	o.logger.Infof("开始上传文件: %s -> %s", localPath, objectKey)
	err = bucket.PutObjectFromFile(objectKey, localPath, options...)
	if err != nil {
		return fmt.Errorf("上传文件失败: %w", err)
	}

	o.logger.Infof("SUCCESS: 文件上传成功 - %s -> oss://%s/%s", localPath, bucketName, objectKey)
	return nil
}

// uploadDirectory 上传目录到OSS
func (o *OSSUploader) uploadDirectory(client *oss_sdk.Client, bucketName, localDir, targetDir string, metas []*resource.KeyVal) error {
	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return fmt.Errorf("获取OSS存储桶失败: %w", err)
	}

	// 遍历目录并上传每个文件
	err = filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// 计算相对路径
		relPath, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}

		// 构造OSS对象键
		objectKey := filepath.Join(targetDir, relPath)
		objectKey = strings.ReplaceAll(objectKey, string(filepath.Separator), "/") // 统一使用正斜杠

		// 构建OSS对象元数据
		options := []oss_sdk.Option{}

		// 添加用户指定的元数据
		for _, meta := range metas {
			if meta.Key == "html" {
				// 特殊处理html元数据，它包含完整的头部信息如"Cache-Control:no-cache"
				parts := strings.SplitN(meta.Value, ":", 2)
				if len(parts) == 2 {
					headerKey := strings.TrimSpace(parts[0])
					headerValue := strings.TrimSpace(parts[1])
					options = append(options, oss_sdk.SetHeader(headerKey, headerValue))
				}
			} else {
				options = append(options, oss_sdk.Meta(meta.Key, meta.Value))
			}
		}

		// 上传文件
		o.logger.Infof("开始上传文件: %s -> %s", path, objectKey)
		err = bucket.PutObjectFromFile(objectKey, path, options...)
		if err != nil {
			return fmt.Errorf("上传文件 %s 失败: %w", path, err)
		}

		o.logger.Infof("SUCCESS: 文件上传成功 - %s -> oss://%s/%s", path, bucketName, objectKey)
		return nil
	})

	return err
}
