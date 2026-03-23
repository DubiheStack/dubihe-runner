package jar

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	buildimage "github.com/DubiheStack/dubihe-runner/engine/buildimage"
	"github.com/DubiheStack/dubihe-runner/resource"
)

// Jar JAR构建器
type Jar struct{}

// Ensure Jar implements buildimage.Builder interface
var _ buildimage.Builder = (*Jar)(nil)

// Builder 是公开的 JAR构建器实例
var Builder = &Jar{}

// init 初始化时注册构建器
func init() {
	buildimage.RegisterBuilder("java:jar", func() buildimage.Builder {
		return &Jar{}
	})
}

// Prepare 准备构建环境
func (jar *Jar) Prepare(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	// 如果有自定义 Dockerfile，使用自定义的
	if config.CustomDockerfile != "" {
		fmt.Println("use custom Dockerfile: " + config.CustomDockerfile)
		content, err := os.ReadFile(config.CustomDockerfile)
		if err != nil {
			return fmt.Errorf("failed to read custom Dockerfile: %w", err)
		}
		// 将自定义 Dockerfile 复制到工作目录
		dockerfilePath := filepath.Join(workDir, "Dockerfile")
		if err := os.WriteFile(dockerfilePath, content, 0644); err != nil {
			return fmt.Errorf("failed to write Dockerfile: %w", err)
		}
		return nil
	}

	// 如果是 SpringBoot 项目，并且是多个文件的情况，走下面的逻辑
	if config.Jar == "" {
		// 这里应该是处理多文件的情况
		return jar.PrepareSpringBootMultifile(config, workDir)
	}

	// 标准 JAR 流程：查询镜像配置并生成 Dockerfile
	if err := buildimage.FixImage(ctx, config); err != nil {
		return fmt.Errorf("failed to get image configuration: %w", err)
	}

	// 复制 JAR 文件到工作目录
	jarName := filepath.Base(config.Jar)
	jarDestPath := filepath.Join(workDir, jarName)

	// 如果 JAR 文件路径是相对路径，需要从工作空间查找
	var jarSourcePath string
	if filepath.IsAbs(config.Jar) {
		jarSourcePath = config.Jar
	} else {
		// 在当前工作目录下查找（Maven构建后的输出目录）
		// 尝试从当前 workDir 的子目录中查找
		searchPath := filepath.Join(workDir, config.Jar)
		if _, err := os.Stat(searchPath); err == nil {
			jarSourcePath = searchPath
		} else {
			// 如果找不到，尝试在 sources 目录下查找
			// 工作目录结构：workspace/Stage/Job，sources 在 workspace/sources/
			workspaceDir := filepath.Dir(filepath.Dir(workDir))
			sourcesDir := filepath.Join(workspaceDir, "sources")
			jarSourcePath = filepath.Join(sourcesDir, config.Jar)

			// 如果还是找不到，可能是多模块项目，需要查找 repo 目录
			if _, err := os.Stat(jarSourcePath); os.IsNotExist(err) {
				// 尝试查找所有 repo 目录
				repoDirs, err := filepath.Glob(filepath.Join(sourcesDir, "*"))
				if err == nil {
					for _, repoDir := range repoDirs {
						potentialPath := filepath.Join(repoDir, config.Jar)
						if _, err := os.Stat(potentialPath); err == nil {
							jarSourcePath = potentialPath
							break
						}
					}
				}
			}
		}
	}

	// 检查源文件是否存在
	if _, err := os.Stat(jarSourcePath); os.IsNotExist(err) {
		return fmt.Errorf("JAR file not found: %s (searched at: %s)", jarName, jarSourcePath)
	}

	// 复制 JAR 文件到工作目录
	if err := copyFile(jarSourcePath, jarDestPath); err != nil {
		return fmt.Errorf("failed to copy JAR file: %w", err)
	}

	fmt.Printf("Copied JAR file from %s to %s\n", jarSourcePath, jarDestPath)

	return buildimage.CreateDockerfile(config, workDir, "jar")
}

// Build 执行构建
func (jar *Jar) Build(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	return buildimage.BuildDockerImage(ctx, config, workDir)
}

// Push 推送构建产物
func (jar *Jar) Push(ctx context.Context, config *resource.BuildImageStep) error {
	return buildimage.PushImage(ctx, config)
}

// SendResult 发送构建结果
func (jar *Jar) SendResult(ctx context.Context, config *resource.BuildImageStep) error {
	return buildimage.SendBuildResult(ctx, config)
}

// PrepareSpringBootMultifile 为 Spring Boot 多文件项目准备构建环境
func (jar *Jar) PrepareSpringBootMultifile(config *resource.BuildImageStep, workDir string) error {
	// TODO: 这里需要实现 Spring Boot 多文件项目的构建逻辑
	// 目前先使用默认的 JAR 处理方式
	ctx := context.Background()
	if err := buildimage.FixImage(ctx, config); err != nil {
		return fmt.Errorf("failed to get image configuration: %w", err)
	}
	return buildimage.CreateDockerfile(config, workDir, "jar")
}

// Command 展示将要执行的命令
func (jar *Jar) Command(config *resource.BuildImageStep) (string, error) {
	return "not support", nil
}

// Clean 清理资源
func (jar *Jar) Clean(config *resource.BuildImageStep) {
	// 实现清理逻辑
}

// FindConflict 查找冲突
func (jar *Jar) FindConflict(config *resource.BuildImageStep) error {
	// ignore
	return nil
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}
