// Package oss 实现OSS上传步骤相关功能
package oss

import (
	"gopkg.in/yaml.v3"

	"github.com/DubiheStack/dubihe-runner/resource"
)

// 为兼容性添加别名
type KeyVal = resource.KeyVal

// OSSUploadStep 阿里云OSS上传步骤参数
type OSSUploadStep struct {
	// ServiceConnection 服务连接ID
	ServiceConnection string `yaml:"serviceConnection,omitempty"`
	// Region 区域
	Region string `yaml:"region,omitempty"`
	// Bucket 存储桶名称
	Bucket string `yaml:"bucket,omitempty"`
	// TargetFilePath 目标文件路径
	TargetFilePath string `yaml:"targetFilePath,omitempty"`
	// SourceFilePath 源文件路径
	SourceFilePath string `yaml:"sourceFilePath,omitempty"`
	// Metas 元数据列表
	Metas []*KeyVal `yaml:"metas,omitempty"`
}

// ParseOSSUploadStep 从interface{}解析OSSUploadStep
func ParseOSSUploadStep(data interface{}) (*OSSUploadStep, error) {
	// 尝试直接转换
	if step, ok := data.(*OSSUploadStep); ok {
		return step, nil
	}

	// 如果是map类型，转换为YAML节点再解析
	if m, ok := data.(map[string]interface{}); ok {
		yamlData, err := yaml.Marshal(m)
		if err != nil {
			return nil, err
		}

		var step OSSUploadStep
		err = yaml.Unmarshal(yamlData, &step)
		if err != nil {
			return nil, err
		}
		return &step, nil
	}

	// 尝试将其作为YAML节点解析
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return nil, err
	}

	var step OSSUploadStep
	err = yaml.Unmarshal(yamlData, &step)
	if err != nil {
		return nil, err
	}
	return &step, nil
}
