// Package resource 定义阿里云云效流水线的资源结构
// 对标阿里云云效Flow的YAML规范
package resource

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Pipeline 表示云效流水线的顶层结构
type Pipeline struct {
	// Version 流水线版本
	Version string `yaml:"version,omitempty"`
	// Name 流水线名称
	Name string `yaml:"name,omitempty"`
	// Trigger 触发器配置
	Trigger *Trigger `yaml:"trigger,omitempty"`
	// Variables 全局变量
	Variables map[string]string `yaml:"variables,omitempty"`
	// Sources 代码源配置
	Sources SourceList `yaml:"sources,omitempty"`
	// DefaultWorkspace 默认工作空间
	DefaultWorkspace string `yaml:"defaultWorkspace,omitempty"`
	// Stages 阶段列表
	Stages StageList `yaml:"stages,omitempty"`
}

// Trigger 触发器配置
type Trigger struct {
	// Push 推送触发
	Push *PushTrigger `yaml:"push,omitempty"`
	// PullRequest 合并请求触发
	PullRequest *PullRequestTrigger `yaml:"pullRequest,omitempty"`
	// Schedule 定时触发
	Schedule *ScheduleTrigger `yaml:"schedule,omitempty"`
}

// PushTrigger 推送触发配置
type PushTrigger struct {
	Branches []string `yaml:"branches,omitempty"`
	Tags     []string `yaml:"tags,omitempty"`
}

// PullRequestTrigger 合并请求触发配置
type PullRequestTrigger struct {
	TargetBranches []string `yaml:"targetBranches,omitempty"`
	SourceBranches []string `yaml:"sourceBranches,omitempty"`
}

// ScheduleTrigger 定时触发配置
type ScheduleTrigger struct {
	Cron string `yaml:"cron,omitempty"`
}

// Source 代码源配置
type Source struct {
	Key           string   `yaml:"key,omitempty"`
	Type          string   `yaml:"type,omitempty"` // codeup, github, gitlab, etc.
	Name          string   `yaml:"name,omitempty"`
	Endpoint      string   `yaml:"endpoint,omitempty"`
	Branch        string   `yaml:"branch,omitempty"`
	Commit        string   `yaml:"commit,omitempty"` // 指定提交ID
	CloneDepth    int      `yaml:"cloneDepth,omitempty"`
	CredentialUID string   `yaml:"credentialUid,omitempty"` // 凭证UID
	TriggerEvents []string `yaml:"triggerEvents,omitempty"`
	Certificate   struct {
		ID   string `yaml:"id,omitempty"`   // 凭证ID
		Type string `yaml:"type,omitempty"` // privatekey/account
	} `yaml:"certificate,omitempty"`
}

// SourceList 代码源列表
type SourceList []*Source

// Index 根据key查找代码源
func (list SourceList) Index(key string) (*Source, bool) {
	for _, s := range list {
		if s.Key == key {
			return s, true
		}
	}
	return nil, false
}

// UnmarshalYAML 自定义YAML反序列化
func (l *SourceList) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("unexpected non-mapping source node")
	}

	var i int
	for i < len(node.Content) {
		srcKey := node.Content[i].Value
		src := &Source{}
		err := node.Content[i+1].Decode(src)
		if err != nil {
			return err
		}
		src.Key = srcKey
		*l = append(*l, src)
		i += 2
	}
	return nil
}

// GetDefaultSource 获取默认代码源
func (p *Pipeline) GetDefaultSource() *Source {
	if p.Sources == nil || len(p.Sources) == 0 {
		return nil
	}
	if p.DefaultWorkspace != "" {
		r, ok := p.Sources.Index(p.DefaultWorkspace)
		if ok {
			return r
		}
		return nil
	}
	return p.Sources[0]
}

// Environ 获取代码源环境变量
func (src *Source) Environ() map[string]string {
	return map[string]string{
		"DUBIHE_REPO_NAME":   src.Name,
		"DUBIHE_GIT_URL":     src.Endpoint,
		"DUBIHE_REPO_BRANCH": src.Branch,
	}
}
