package resource

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Job 表示一个任务
type Job struct {
	Key    string `yaml:"key,omitempty"`
	Name   string `yaml:"name,omitempty"`
	RunsOn struct {
		// Pool 运行池: 公共资源池或私有资源池
		Pool string `yaml:"pool,omitempty"`
		// Labels 标签选择器
		Labels []string `yaml:"labels,omitempty"`
		// Container 容器镜像 (Docker模式)
		Container string `yaml:"container,omitempty"`
		// DockerFile 使用Dockerfile构建环境
		DockerFile string `yaml:"dockerfile,omitempty"`
		// Self 自托管环境标识
		Self bool `yaml:"self,omitempty"`
	} `yaml:"runsOn,omitempty"`
	// Needs 依赖的任务
	Needs []string `yaml:"needs,omitempty"`
	// Condition 执行条件
	Condition string `yaml:"condition,omitempty"`
	// Strategy 策略配置
	Strategy *Strategy `yaml:"strategy,omitempty"`
	// Services 服务容器
	Services []*Service `yaml:"services,omitempty"`
	// Steps 步骤列表
	Steps StepList `yaml:"steps,omitempty"`
	// Timeout 超时时间(分钟)
	Timeout int `yaml:"timeout,omitempty"`
	// ContinueOnError 失败时是否继续
	ContinueOnError bool `yaml:"continueOnError,omitempty"`
	// Env 环境变量
	Env map[string]string `yaml:"env,omitempty"`
}

// Strategy 任务执行策略
type Strategy struct {
	// Matrix 矩阵构建
	Matrix map[string][]string `yaml:"matrix,omitempty"`
	// FailFast 快速失败
	FailFast bool `yaml:"failFast,omitempty"`
	// MaxParallel 最大并行数
	MaxParallel int `yaml:"maxParallel,omitempty"`
}

// Service 服务容器
type Service struct {
	Name    string            `yaml:"name,omitempty"`
	Image   string            `yaml:"image,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
	Ports   []string          `yaml:"ports,omitempty"`
	Options string            `yaml:"options,omitempty"`
}

// JobList 任务列表
type JobList []*Job

// Index 根据key查找任务
func (list JobList) Index(key string) (*Job, bool) {
	for _, job := range list {
		if job.Key == key {
			return job, true
		}
	}
	return nil, false
}

// IsDocker 判断是否是Docker模式
func (j *Job) IsDocker() bool {
	return j.RunsOn.Container != ""
}

// UnmarshalYAML 自定义YAML反序列化
func (l *JobList) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("unexpected non-mapping job node")
	}

	var i int
	for i < len(node.Content) {
		jobKey := node.Content[i].Value
		job := &Job{}
		err := node.Content[i+1].Decode(job)
		if err != nil {
			return err
		}

		job.Key = jobKey
		*l = append(*l, job)
		i += 2
	}
	return nil
}
