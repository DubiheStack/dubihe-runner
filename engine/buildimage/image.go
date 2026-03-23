package buildimage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/DubiheStack/dubihe-runner/resource"
	"github.com/sirupsen/logrus"
)

const (
	// getTemplateByProjectNameAndEnvTypeAndCluster 根据项目名称、环境和集群获取镜像模板
	getTemplateByProjectNameAndEnvTypeAndCluster = "%s/api/v1/dockerTemplate/getTemplateByBake?projectName=%s&envType=%s&cluster=%s"
	// getTemplateByProjectNameAndEnvType 根据项目名称和环境获取镜像模板
	getTemplateByProjectNameAndEnvType = "%s/api/v1/dockerTemplate/getTemplateByBake?projectName=%s&envType=%s"
)

// FixImage 处理镜像配置，查询或设置合适的镜像
func FixImage(ctx context.Context, config *resource.BuildImageStep) error {
	log := logrus.WithField("module", "buildimage")

	// 优先使用自定义镜像
	if config.UseCustomImage && len(config.ImageFrom) > 0 && len(config.ImageTo) > 0 {
		log.Info("use custom image configuration")
		addTag(config)
		return nil
	}

	// 从服务器获取镜像模板
	var url string
	image := &ImageInfo{}

	if len(config.Cluster) > 0 {
		url = fmt.Sprintf(getTemplateByProjectNameAndEnvTypeAndCluster,
			config.ServerUrl,
			config.AppName,
			config.Environment,
			config.Cluster)
	} else {
		url = fmt.Sprintf(getTemplateByProjectNameAndEnvType,
			config.ServerUrl,
			config.AppName,
			config.Environment)
	}

	log.WithField("url", url).Info("querying image template from server")

	if err := httpGet(ctx, url, image); err != nil {
		return fmt.Errorf("failed to query image template: %w", err)
	}

	// 判断平台类型
	var platform string
	if len(config.PackageVersion) > 0 {
		platform = "paas"
	} else {
		platform = "cmdb-docker"
	}

	log.WithField("platform", platform).Info("detected platform type")

	// 保存查询到的镜像配置
	config.ImageFrom = image.From
	config.ImageTo = image.To
	config.DockerFileTemplate = image.DockerFileTemplate
	config.DockerIgnore = image.DockerIgnore

	if len(image.From) > 0 && len(image.To) > 0 {
		addTag(config)
		return nil
	}

	return fmt.Errorf("query image for app '%s' from platform '%s' error: empty, url = %s",
		config.AppName, platform, url)
}

// addTag 为镜像添加时间戳标签
func addTag(config *resource.BuildImageStep) {
	if !strings.Contains(config.ImageTo, ":") {
		// 自动以时间戳为 image 打 tag
		imageTag := time.Now().Format("20060102-150405")
		config.ImageTo = config.ImageTo + ":" + imageTag
		logrus.WithField("image", config.ImageTo).Info("added timestamp tag to image")
	}
}

// ImageInfo 镜像信息结构体
type ImageInfo struct {
	From               string `json:"from"`
	To                 string `json:"to"`
	DockerFileTemplate string `json:"dockerFileTemplate"`
	DockerIgnore       string `json:"dockerIgnore"`
}

// httpGet 发送 HTTP GET 请求
func httpGet(ctx context.Context, url string, result interface{}) error {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API returned error status %d", resp.StatusCode)
	}

	// 使用通用 HTTP 工具解析响应
	return decodeJSONResponse(resp.Body, result)
}

// decodeJSONResponse 解码 JSON 响应
func decodeJSONResponse(body io.Reader, result interface{}) error {
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(result); err != nil {
		return fmt.Errorf("failed to decode JSON response: %w", err)
	}
	return nil
}
