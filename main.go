package main

import (
	"fmt"
	"os"

	"github.com/DubiheStack/dubihe-runner/client"
	"github.com/DubiheStack/dubihe-runner/cmd"
	"github.com/DubiheStack/dubihe-runner/license"
)

func main() {
	// 从配置文件加载配置
	configFile := "config.yaml"
	cfg, err := client.LoadConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// 初始化并检查license，使用配置中的license路径
	licensePath := cfg.GetLicensePath()
	checker := license.GetInstanceWithConfigPath(licensePath)

	// 检查License是否过期
	expired, err := func() (bool, error) {
		// 先获取license信息，检查是否真的有license文件
		licenseInfo := checker.GetLicenseInfo()
		if licenseInfo == nil {
			// 没有license文件，跳过检查
			return false, nil
		}

		// 检查是否有必要的license信息
		if licenseInfo.GetLimitDays() == "" && licenseInfo.GetStartDate() == "" {
			// 没有设置时间限制，跳过检查
			return false, nil
		}

		// 正确调用IsExpired函数并返回两个值
		expired := license.IsExpired()
		return expired, nil
	}()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking license expiration: %v\n", err)
		os.Exit(1)
	}

	if expired {
		fmt.Fprintf(os.Stderr, "Error: License has expired!\n")
		os.Exit(1)
	}

	// 在开发模式下跳过服务器信息验证
	if !cfg.DevMode {
		// 检查服务器信息是否匹配
		if !license.IsSameServerAuto() {
			fmt.Fprintf(os.Stderr, "Error: Server information does not match the license!\n")
			os.Exit(1)
		}
	} else {
		fmt.Println("Warning: Skipping server information validation in development mode")
	}

	// 显示license信息
	licenseInfo := checker.GetLicenseInfo()
	if licenseInfo != nil {
		if licenseInfo.GetLimitDays() != "" && licenseInfo.GetStartDate() != "" {
			endTime, err := license.GetEndTime()
			if err == nil {
				fmt.Printf("License verified successfully. Expires on: %s\n", endTime)
			} else {
				fmt.Printf("License verified successfully.\n")
			}
		} else {
			fmt.Printf("License verified successfully (no expiration).\n")
		}
	} else {
		fmt.Printf("Running without license.\n")
	}

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
