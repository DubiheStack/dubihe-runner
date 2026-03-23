package resource

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// 步骤类型常量
const (
	StepTypeScript            = "Command"           // 脚本执行
	StepTypeCheckout          = "checkout"          // 代码检出
	StepTypeUpload            = "upload"            // 上传制品
	StepTypeDownload          = "download"          // 下载制品
	StepTypeCache             = "cache"             // 缓存
	StepTypePlugin            = "plugin"            // 插件
	StepTypeOSSUpload         = "OSSUpload"         // 阿里云OSS上传
	StepTypeBuildImage        = "buildImage"        // 制作镜像
	StepTypeArtifactUploadOss = "artifactUploadOss" // 构件上传OSS
)

// Step 表示一个执行步骤
type Step struct {
	Key  string `yaml:"key,omitempty"`
	Name string `yaml:"name,omitempty"`
	// Step 步骤类型: script, checkout, upload, download, cache, plugin
	Step string `yaml:"step,omitempty"`
	// Uses 使用的插件 (plugin模式)
	Uses string `yaml:"uses,omitempty"`
	// With 步骤参数
	With interface{} `yaml:"with,omitempty"`
	// Env 环境变量
	Env map[string]string `yaml:"env,omitempty"`
	// Condition 执行条件
	Condition string `yaml:"condition,omitempty"`
	// ContinueOnError 失败时是否继续
	ContinueOnError bool `yaml:"continueOnError,omitempty"`
	// Timeout 超时时间(分钟)
	Timeout int `yaml:"timeout,omitempty"`
	// WorkingDirectory 工作目录
	WorkingDirectory string `yaml:"workingDirectory,omitempty"`
}

// ScriptStep 脚本步骤参数
type ScriptStep struct {
	// Run 要执行的脚本
	Run string `yaml:"run,omitempty"`
	// Shell 使用的shell类型: bash, sh, pwsh, powershell, cmd
	Shell string `yaml:"shell,omitempty"`
	// Variables 变量列表
	Variables []*KeyVal `yaml:"variables,omitempty"`
}

// CheckoutStep 代码检出步骤参数
type CheckoutStep struct {
	// Repo 代码源key
	Repo string `yaml:"repo,omitempty"`
	// Ref 分支/标签/commit
	Ref string `yaml:"ref,omitempty"`
	// FetchDepth 克隆深度
	FetchDepth int `yaml:"fetchDepth,omitempty"`
	// Submodules 是否拉取子模块
	Submodules bool `yaml:"submodules,omitempty"`
}

// UploadStep 上传制品步骤参数
type UploadStep struct {
	// Name 制品名称
	Name string `yaml:"name,omitempty"`
	// Path 上传路径
	Path string `yaml:"path,omitempty"`
}

// DownloadStep 下载制品步骤参数
type DownloadStep struct {
	// Name 制品名称
	Name string `yaml:"name,omitempty"`
	// Path 下载路径
	Path string `yaml:"path,omitempty"`
}

// CacheStep 缓存步骤参数
type CacheStep struct {
	// Key 缓存key
	Key string `yaml:"key,omitempty"`
	// Path 缓存路径
	Path string `yaml:"path,omitempty"`
}

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

// BuildImageStep 制作镜像步骤参数
type BuildImageStep struct {
	// Language 编程语言
	Language string `yaml:"language,omitempty"`
	// Tool 构建工具 (maven, gradle, jar, war)
	Tool string `yaml:"tool,omitempty"`
	// Jar JAR 文件路径
	Jar string `yaml:"jar,omitempty"`
	// War WAR 文件路径
	War string `yaml:"war,omitempty"`
	// Operator 操作员
	Operator string `yaml:"operator,omitempty"`
	// PackageVersion 包版本
	PackageVersion string `yaml:"packageVersion,omitempty"`
	// NoahPublishBillId 发布单 ID
	NoahPublishBillId int64 `yaml:"publishBillId,omitempty"`
	// CallBackUrl 回调地址
	CallBackUrl string `yaml:"callBackUrl,omitempty"`
	// ServerUrl 服务器地址
	ServerUrl string `yaml:"serverUrl,omitempty"`
	// Environment 环境
	Environment string `yaml:"environment,omitempty"`
	// CustomDockerfile 自定义 Dockerfile 路径
	CustomDockerfile string `yaml:"customDockerfile,omitempty"`
	// SkipPrepare 跳过准备阶段
	SkipPrepare bool `yaml:"skipPrepare,omitempty"`
	// AppName 应用名称
	AppName string `yaml:"appName,omitempty"`
	// Cluster 集群
	Cluster string `yaml:"cluster,omitempty"`
	// UseCustomImage 是否使用自定义镜像
	UseCustomImage bool `yaml:"useCustomImage,omitempty"`
	// ImageFrom 基础镜像 (由查询后赋值)
	ImageFrom string `yaml:"-"`
	// ImageTo 目标镜像 (由查询后赋值)
	ImageTo string `yaml:"-"`
	// DockerFileTemplate Dockerfile 模板
	DockerFileTemplate string `yaml:"-"`
	// DockerIgnore .dockerignore 模板
	DockerIgnore string `yaml:"-"`
}

// ArtifactUploadOssStep 构件上传OSS步骤参数
type ArtifactUploadOssStep struct {
	// CodePath 代码文件路径
	CodePath string `yaml:"codePath,omitempty"`
	// CallBackUrl 回调地址
	CallBackUrl string `yaml:"callBackUrl,omitempty"`
	// PackageVersion 包版本
	PackageVersion string `yaml:"packageVersion,omitempty"`
	// NoahPublishBillId 发布单ID
	NoahPublishBillId int64 `yaml:"publishBillId,omitempty"`
	// AppName 应用名称（模块名）
	AppName string `yaml:"appName,omitempty"`
	// Operator 操作员
	Operator string `yaml:"operator,omitempty"`
	// Environment 环境
	Environment string `yaml:"environment,omitempty"`
	// OssType OSS类型 (aliyun, tencent)
	OssType string `yaml:"ossType,omitempty"`
	// AccessKeyId 访问密钥ID
	AccessKeyId string `yaml:"accessKeyId,omitempty"`
	// AccessKeySecret 访问密钥
	AccessKeySecret string `yaml:"accessKeySecret,omitempty"`
	// Endpoint 端点
	Endpoint string `yaml:"endpoint,omitempty"`
	// BucketName 存储桶名称
	BucketName string `yaml:"bucketName,omitempty"`
	// BathUrl 访问URL
	BathUrl string `yaml:"bathUrl,omitempty"`
}

// KeyVal 键值对
type KeyVal struct {
	Key   string `yaml:"key,omitempty"`
	Value string `yaml:"value,omitempty"`
}

// StepList 步骤列表
type StepList []*Step

// Index 根据key查找步骤
func (list StepList) Index(key string) (*Step, bool) {
	for _, step := range list {
		if step.Key == key {
			return step, true
		}
	}
	return nil, false
}

// UnmarshalYAML 自定义YAML反序列化
func (v *Step) UnmarshalYAML(node *yaml.Node) error {
	var withNode *yaml.Node
	for i, n := range node.Content {
		if n.Value == "with" {
			withNode = node.Content[i+1]
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			break
		}
	}

	type S Step
	obj := (*S)(v)
	err := node.Decode(obj)
	if err != nil {
		return err
	}

	// 根据步骤类型解析with参数
	if withNode != nil {
		switch v.Step {
		case StepTypeScript: // 现在只支持Command类型
			v.With = new(ScriptStep)
		case StepTypeCheckout:
			v.With = new(CheckoutStep)
		case StepTypeUpload:
			v.With = new(UploadStep)
		case StepTypeDownload:
			v.With = new(DownloadStep)
		case StepTypeCache:
			v.With = new(CacheStep)
		case StepTypeOSSUpload:
			v.With = new(OSSUploadStep)
		case StepTypeBuildImage:
			v.With = new(BuildImageStep)
		case StepTypeArtifactUploadOss:
			v.With = new(ArtifactUploadOssStep)
		default:
			// 对于插件类型,使用map存储参数
			v.With = make(map[string]interface{})
		}
		return withNode.Decode(v.With)
	}

	return nil
}

// UnmarshalYAML 步骤列表反序列化
func (s *StepList) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		// 如果不是映射节点，尝试作为列表节点处理（保持向后兼容）
		if node.Kind == yaml.SequenceNode {
			// 处理列表格式的steps
			for _, item := range node.Content {
				step := &Step{}
				err := item.Decode(step)
				if err != nil {
					return err
				}

				// 如果step没有key但有name，使用name作为key
				if step.Key == "" && step.Name != "" {
					step.Key = step.Name
				}

				*s = append(*s, step)
			}
			return nil
		}
		return fmt.Errorf("unexpected non-mapping step node")
	}

	// 处理映射格式的steps（阿里云效格式）
	var i int
	for i < len(node.Content) {
		stepKey := node.Content[i].Value
		step := &Step{}
		err := node.Content[i+1].Decode(step)
		if err != nil {
			return err
		}

		step.Key = stepKey
		*s = append(*s, step)
		i += 2
	}
	return nil
}

// GetScript 获取脚本步骤的脚本内容
func (s *Step) GetScript() string {
	if script, ok := s.With.(*ScriptStep); ok {
		return script.Run
	}
	return ""
}

// GetShell 获取脚本步骤的shell类型
func (s *Step) GetShell() string {
	if script, ok := s.With.(*ScriptStep); ok {
		if script.Shell != "" {
			return script.Shell
		}
	}
	return "bash" // 默认bash
}
