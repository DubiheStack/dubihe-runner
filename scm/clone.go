// Package scm 提供源代码管理功能
package scm

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/DubiheStack/dubihe-runner/client"
	"github.com/sirupsen/logrus"
)

// CloneOptions 克隆选项
type CloneOptions struct {
	// URL 仓库地址
	URL string
	// Branch 分支名称
	Branch string
	// Commit 提交ID
	Commit string
	// Depth 克隆深度，0表示完整克隆
	Depth int
	// Dir 目标目录
	Dir string
	// CredentialUID 凭证UID
	CredentialUID string
	// Token 令牌认证
	Token string
	// Username 用户名（用于密码认证）
	Username string
	// Password 密码（用于密码认证）
	Password string
	// PrivateKey 私钥（用于SSH认证）
	PrivateKey string

	// 为了向后兼容，保留旧字段名
	RepoURL   string `yaml:"-"` // 不参与YAML序列化
	TargetDir string `yaml:"-"` // 不参与YAML序列化
}

// Cloner 代码克隆器
type Cloner struct {
	client *client.Client
	logger *logrus.Entry
}

// NewCloner 创建克隆器
func NewCloner(c *client.Client) *Cloner {
	return &Cloner{
		client: c,
		logger: logrus.WithField("component", "scm"),
	}
}

// Clone 克隆代码
func (c *Cloner) Clone(ctx context.Context, opts *CloneOptions) error {
	// 为了向后兼容，处理旧字段名
	if opts.URL == "" && opts.RepoURL != "" {
		opts.URL = opts.RepoURL
	}
	if opts.Dir == "" && opts.TargetDir != "" {
		opts.Dir = opts.TargetDir
	}

	c.logger.WithFields(logrus.Fields{
		"url":    opts.URL,
		"branch": opts.Branch,
		"target": opts.Dir,
	}).Info("[SCM] 开始克隆代码...")

	// 获取凭证
	var cred *client.Credential
	var err error
	if opts.CredentialUID != "" && c.client != nil {
		c.logger.WithFields(logrus.Fields{
			"credentialUID": opts.CredentialUID,
		}).Debug("[SCM] 正在获取凭证信息...")

		cred, err = c.client.GetCredential(ctx, opts.CredentialUID)
		if err != nil {
			c.logger.WithError(err).WithFields(logrus.Fields{
				"credentialUID": opts.CredentialUID,
			}).Error("[SCM] 获取凭证失败")
			return fmt.Errorf("获取凭证失败: %w", err)
		}

		c.logger.WithFields(logrus.Fields{
			"type": cred.Type,
			"name": cred.Name,
		}).Debug("[SCM] 获取凭证成功")
	}

	// 如果直接提供了凭证信息，优先使用
	if opts.Token != "" || opts.Username != "" || opts.Password != "" || opts.PrivateKey != "" {
		c.logger.Debug("[SCM] 使用直接提供的凭证信息")
	} else if cred != nil {
		// 使用从服务端获取的凭证信息
		c.logger.Debug("[SCM] 使用从服务端获取的凭证信息")
		switch cred.Type {
		case "token":
			opts.Token = cred.Token
			c.logger.Debug("[SCM] 设置Token凭证")
		case "password":
			opts.Username = cred.UserName
			opts.Password = cred.Password
			c.logger.Debug("[SCM] 设置用户名密码凭证")
		case "ssh":
			opts.PrivateKey = cred.PrivateKey
			c.logger.Debug("[SCM] 设置SSH私钥凭证")
		default:
			c.logger.WithFields(logrus.Fields{
				"type": cred.Type,
			}).Warn("[SCM] 未知的凭证类型")
		}
	} else {
		c.logger.Debug("[SCM] 未提供凭证信息，将使用匿名方式克隆")
	}

	// 构建带凭证的URL
	c.logger.Debug("[SCM] 正在构建认证URL...")
	authURL, err := c.buildAuthURL(opts.URL, opts)
	if err != nil {
		c.logger.WithError(err).Error("[SCM] 构建认证URL失败")
		return fmt.Errorf("构建认证URL失败: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"authURL": maskURL(authURL),
	}).Debug("[SCM] 认证URL构建完成")

	// 确保目标目录存在
	c.logger.Debug("[SCM] 正在确保目标目录存在...")
	if err := os.MkdirAll(filepath.Dir(opts.Dir), 0755); err != nil {
		c.logger.WithError(err).Error("[SCM] 创建目录失败")
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 执行git clone
	c.logger.Debug("[SCM] 正在执行git clone...")
	if err := c.gitClone(ctx, authURL, opts); err != nil {
		c.logger.WithError(err).Error("[SCM] 克隆失败")
		return fmt.Errorf("克隆失败: %w", err)
	}

	// 如果指定了commit，切换到指定commit
	if opts.Commit != "" {
		c.logger.WithFields(logrus.Fields{
			"commit": opts.Commit,
		}).Debug("[SCM] 正在切换到指定commit...")

		if err := c.gitCheckout(ctx, opts.Dir, opts.Commit); err != nil {
			c.logger.WithError(err).Error("[SCM] 切换commit失败")
			return fmt.Errorf("切换commit失败: %w", err)
		}

		c.logger.WithFields(logrus.Fields{
			"commit": opts.Commit,
		}).Debug("[SCM] 切换commit完成")
	}

	c.logger.Info("[SCM] 代码克隆成功")
	return nil
}

// BuildAuthURL 构建带认证信息的URL (公开方法)
func (c *Cloner) BuildAuthURL(repoURL string, opts *CloneOptions) (string, error) {
	return c.buildAuthURL(repoURL, opts)
}

// buildAuthURL 构建带认证信息的URL
func (c *Cloner) buildAuthURL(repoURL string, opts *CloneOptions) (string, error) {
	c.logger.Debugf("[SCM] buildAuthURL called with repoURL: %s, Token: %v, Username: %v, PrivateKey: %v",
		repoURL, opts.Token != "", opts.Username != "", opts.PrivateKey != "")

	// 如果没有凭证信息，直接返回原始URL
	if opts.Token == "" && opts.Username == "" && opts.Password == "" && opts.PrivateKey == "" {
		c.logger.Debug("[SCM] No credentials provided, returning original URL")
		return repoURL, nil
	}

	// 对于SSH URL (git@host:path)，如果使用SSH私钥认证，直接返回原始URL
	// SSH URL格式不能被url.Parse正确解析
	if strings.HasPrefix(repoURL, "git@") && opts.PrivateKey != "" {
		c.logger.Debug("[SCM] SSH URL with private key, returning original URL")
		return repoURL, nil
	}

	// 根据凭证类型构建认证URL
	if opts.Token != "" {
		c.logger.Debug("[SCM] Building token URL")
		return c.buildTokenURL(repoURL, opts)
	} else if opts.Username != "" && opts.Password != "" {
		c.logger.Debug("[SCM] Building password URL")
		return c.buildPasswordURL(repoURL, opts)
	} else if opts.PrivateKey != "" {
		// SSH认证不修改URL，通过环境变量或SSH agent处理
		c.logger.Debug("[SCM] SSH private key provided, returning original URL")
		return repoURL, nil
	}

	c.logger.Debug("[SCM] Returning original URL")
	return repoURL, nil
}

// buildTokenURL 构建token认证URL
func (c *Cloner) buildTokenURL(repoURL string, opts *CloneOptions) (string, error) {
	// 对于SSH URL (git@host:path)，不能使用token认证，直接返回原始URL
	if strings.HasPrefix(repoURL, "git@") {
		c.logger.Debug("[SCM] SSH URL cannot use token authentication, returning original URL")
		return repoURL, nil
	}

	u, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("解析URL失败: %w", err)
	}

	// 对于GitLab，使用 oauth2:token 格式
	// 对于GitHub，使用 token:x-oauth-basic 格式
	if strings.Contains(u.Host, "github") {
		u.User = url.UserPassword(opts.Token, "x-oauth-basic")
	} else {
		// GitLab 和其他Git服务器
		u.User = url.UserPassword("oauth2", opts.Token)
	}

	return u.String(), nil
}

// buildPasswordURL 构建用户名密码认证URL
func (c *Cloner) buildPasswordURL(repoURL string, opts *CloneOptions) (string, error) {
	// 对于SSH URL (git@host:path)，不能使用用户名密码认证，直接返回原始URL
	if strings.HasPrefix(repoURL, "git@") {
		c.logger.Debug("[SCM] SSH URL cannot use password authentication, returning original URL")
		return repoURL, nil
	}

	u, err := url.Parse(repoURL)
	if err != nil {
		return "", fmt.Errorf("解析URL失败: %w", err)
	}

	u.User = url.UserPassword(opts.Username, opts.Password)
	return u.String(), nil
}

// gitClone 执行git clone命令
func (c *Cloner) gitClone(ctx context.Context, authURL string, opts *CloneOptions) error {
	// 如果目标目录已存在，先删除它
	if _, err := os.Stat(opts.Dir); err == nil {
		c.logger.Debugf("[SCM] 目标目录 %s 已存在，正在删除...", opts.Dir)
		if err := os.RemoveAll(opts.Dir); err != nil {
			return fmt.Errorf("删除已存在的目录失败: %w", err)
		}
		c.logger.Debugf("[SCM] 已删除目录 %s", opts.Dir)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查目标目录状态失败: %w", err)
	}

	args := []string{"clone"}

	// 添加分支参数
	if opts.Branch != "" {
		args = append(args, "-b", opts.Branch)
	}

	// 添加深度参数
	if opts.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", opts.Depth))
	}

	// 添加URL和目标目录
	args = append(args, authURL, opts.Dir)

	// 创建临时目录用于存放SSH密钥（如果需要）
	tempDir := ""
	var tempFiles []string

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0", // 禁止交互式提示
	)

	// 如果提供了私钥，设置SSH密钥
	if opts.PrivateKey != "" {
		// 创建临时目录
		var err error
		tempDir, err = os.MkdirTemp("", "dubihe-runner-ssh")
		if err != nil {
			return fmt.Errorf("创建临时目录失败: %w", err)
		}
		defer os.RemoveAll(tempDir) // 清理临时目录

		// 创建私钥文件
		privateKeyFile := filepath.Join(tempDir, "id_rsa")
		if err := os.WriteFile(privateKeyFile, []byte(opts.PrivateKey), 0600); err != nil {
			return fmt.Errorf("写入私钥文件失败: %w", err)
		}
		tempFiles = append(tempFiles, privateKeyFile)

		// 设置SSH相关环境变量
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null", privateKeyFile),
		)
	}

	// 记录日志时隐藏敏感信息
	safeArgs := make([]string, len(args))
	copy(safeArgs, args)
	safeArgs[len(safeArgs)-2] = maskURL(authURL)
	c.logger.WithField("args", safeArgs).Debug("[SCM] 执行git clone")

	output, err := cmd.CombinedOutput()
	if err != nil {
		c.logger.WithError(err).WithField("output", string(output)).Error("[SCM] git clone执行失败")
		return fmt.Errorf("git clone失败: %s", string(output))
	}

	return nil
}

// gitCheckout 切换到指定commit
func (c *Cloner) gitCheckout(ctx context.Context, dir, commit string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", commit)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		c.logger.WithError(err).WithField("output", string(output)).Error("[SCM] git checkout执行失败")
		return fmt.Errorf("git checkout失败: %s", string(output))
	}

	c.logger.WithField("commit", commit).Info("[SCM] 切换到指定commit")
	return nil
}

// maskURL 隐藏URL中的敏感信息
func maskURL(rawURL string) string {
	// 对于SSH URL (git@host:path)，直接返回原始URL
	// SSH URL格式不能被url.Parse正确解析
	if strings.HasPrefix(rawURL, "git@") {
		return rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		// 如果解析失败，返回原URL的一部分作为安全措施
		if len(rawURL) > 10 {
			return rawURL[:10] + "..."
		}
		return rawURL
	}

	if u.User != nil {
		u.User = url.UserPassword("****", "****")
	}

	return u.String()
}

// Pull 拉取最新代码
func (c *Cloner) Pull(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "git", "pull")
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull失败: %s", string(output))
	}

	c.logger.Info("[SCM] 代码拉取成功")
	return nil
}
