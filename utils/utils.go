// Package utils 提供通用工具函数
package utils

import (
	"os"
	"path/filepath"
)

// HomePath 获取用户主目录
func HomePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp"
	}
	return home
}

// WorkspacePath 获取工作空间路径
func WorkspacePath(subPath ...string) string {
	base := filepath.Join(HomePath(), ".dubihe")
	if len(subPath) > 0 {
		return filepath.Join(append([]string{base}, subPath...)...)
	}
	return base
}

// EnsureDir 确保目录存在
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// FileExists 检查文件是否存在
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsDir 检查路径是否是目录
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// FindCodeSourceDir 查找代码源目录，优先查找包含pom.xml或package.json等项目文件的目录
func FindCodeSourceDir(workspaceDir string) string {
	sourcesDir := filepath.Join(workspaceDir, "sources")
	
	// 检查sources目录是否存在
	if !IsDir(sourcesDir) {
		return ""
	}
	
	// 遍历sources目录下的所有子目录
	entries, err := os.ReadDir(sourcesDir)
	if err != nil {
		return ""
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			sourceDir := filepath.Join(sourcesDir, entry.Name())
			
			// 检查该目录是否包含常见的项目文件
			if FileExists(filepath.Join(sourceDir, "pom.xml")) || 
			   FileExists(filepath.Join(sourceDir, "package.json")) ||
			   FileExists(filepath.Join(sourceDir, "build.gradle")) ||
			   FileExists(filepath.Join(sourceDir, "go.mod")) ||
			   FileExists(filepath.Join(sourceDir, "requirements.txt")) {
				return sourceDir
			}
			
			// 如果直接目录下没有项目文件，递归查找
			if subDir := findProjectDirRecursively(sourceDir); subDir != "" {
				return subDir
			}
		}
	}
	
	return ""
}

// findProjectDirRecursively 递归查找包含项目文件的目录
func findProjectDirRecursively(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			subDir := filepath.Join(dir, entry.Name())
			
			// 检查该子目录是否包含项目文件
			if FileExists(filepath.Join(subDir, "pom.xml")) || 
			   FileExists(filepath.Join(subDir, "package.json")) ||
			   FileExists(filepath.Join(subDir, "build.gradle")) ||
			   FileExists(filepath.Join(subDir, "go.mod")) ||
			   FileExists(filepath.Join(subDir, "requirements.txt")) {
				return subDir
			}
			
			// 递归查找更深层目录
			if result := findProjectDirRecursively(subDir); result != "" {
				return result
			}
		}
	}
	
	return ""
}