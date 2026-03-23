package buildimage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/DubiheStack/dubihe-runner/resource"
)

// BuildDockerImage 构建Docker镜像
func BuildDockerImage(ctx context.Context, config *resource.BuildImageStep, workDir string) error {
	// 使用查询到的 ImageTo 作为镜像标签
	if config.ImageTo == "" {
		return fmt.Errorf("image target is empty, please check image configuration")
	}

	// 构建命令参数
	cmdArgs := []string{"build", "--pull", "--tag", config.ImageTo, "."}

	// 执行 docker build 命令
	cmd := exec.CommandContext(ctx, "docker", cmdArgs...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		return fmt.Errorf("docker build failed: %w, stderr: %s", err, stderr.String())
	}

	fmt.Println(stdout.String())
	return nil
}

// PushImage 推送镜像到仓库
func PushImage(ctx context.Context, config *resource.BuildImageStep) error {
	// 使用查询到的 ImageTo 作为镜像标签
	if config.ImageTo == "" {
		return fmt.Errorf("image target is empty, please check image configuration")
	}

	// 执行 docker push 命令
	cmd := exec.CommandContext(ctx, "docker", "push", config.ImageTo)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		return fmt.Errorf("docker push failed: %w, stderr: %s", err, stderr.String())
	}

	fmt.Println(stdout.String())
	return nil
}

// SendBuildResult 发送构建结果到 API
func SendBuildResult(ctx context.Context, config *resource.BuildImageStep) error {
	// 准备 POST 数据（使用 form-urlencoded 格式）
	data := fmt.Sprintf(
		"noahPublishId=%d&appName=%s&imageTo=%s&imageFrom=%s&deployOperator=%s&packageVersion=%s&env=%s",
		config.NoahPublishBillId,
		config.AppName,
		config.ImageTo,
		config.ImageFrom,
		config.Operator,
		config.PackageVersion,
		config.Environment,
	)

	// 发送 HTTP POST 请求
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", config.CallBackUrl, strings.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API returned error status %d: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("Build result callback sent successfully: %s\n", string(body))
	return nil
}

// CreateDockerfile 创建Dockerfile
func CreateDockerfile(config *resource.BuildImageStep, workDir string, buildType string) error {
	var dockerfileContent string
	var dockerignoreContent string

	// 如果有自定义 Dockerfile 模板，使用模板渲染
	if config.DockerFileTemplate != "" {
		dockerfileContent = renderDockerfileTemplate(config.DockerFileTemplate, config)
	} else {
		// 否则使用默认模板
		switch buildType {
		case "jar":
			dockerfileContent = createJarDockerfile(config)
			// .dockerignore 需要包含指定的 JAR 文件
			jarName := filepath.Base(config.Jar)
			dockerignoreContent = fmt.Sprintf(".*\n!%s\n", jarName)
		case "war":
			dockerfileContent = createWarDockerfile(config)
			dockerignoreContent = ".*\n!*.war\n"
		case "maven", "gradle":
			// 对于 maven 和 gradle，可能需要更复杂的构建流程
			dockerfileContent = createJavaBuildDockerfile(config, buildType)
			dockerignoreContent = "**/*\n!src/\n!pom.xml\n"
		case "python":
			dockerfileContent = createPythonDockerfile(config)
			dockerignoreContent = ".*\n!*.py\n!requirements.txt\n"
		case "node":
			dockerfileContent = createNodeDockerfile(config)
			dockerignoreContent = ".*\n!*.js\n!package.json\n!package-lock.json\n"
		case "php":
			dockerfileContent = createPhpDockerfile(config)
			dockerignoreContent = ".*\n!*.php\n!composer.json\n"
		case "golang":
			dockerfileContent = createGolangDockerfile(config)
			dockerignoreContent = ".*\n!*.go\n!go.mod\n!go.sum\n"
		case "scala":
			dockerfileContent = createScalaDockerfile(config)
			dockerignoreContent = ".*\n!*.scala\n!build.sbt\n"
		default:
			return fmt.Errorf("unsupported build type: %s", buildType)
		}
	}

	// 如果配置中有 DockerIgnore 模板，使用配置的
	if config.DockerIgnore != "" {
		dockerignoreContent = renderDockerignoreTemplate(config.DockerIgnore, config)
	}

	// 写入 Dockerfile
	dockerfilePath := filepath.Join(workDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		return fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	// 写入.dockerignore
	dockerignorePath := filepath.Join(workDir, ".dockerignore")
	if err := os.WriteFile(dockerignorePath, []byte(dockerignoreContent), 0644); err != nil {
		return fmt.Errorf("failed to write .dockerignore: %w", err)
	}

	return nil
}

// createJarDockerfile 为JAR包创建Dockerfile
func createJarDockerfile(config *resource.BuildImageStep) string {
	javaBaseImage := "openjdk:8-jre-alpine"
	if strings.Contains(strings.ToLower(config.Jar), "spring-boot") {
		javaBaseImage = "openjdk:11-jre-slim"
	}

	return fmt.Sprintf(`FROM %s
COPY %s /app.jar
EXPOSE 8080
ENTRYPOINT ["java", "-jar", "/app.jar"]
`, javaBaseImage, filepath.Base(config.Jar))
}

// createWarDockerfile 为WAR包创建Dockerfile
func createWarDockerfile(config *resource.BuildImageStep) string {
	tomcatBaseImage := "tomcat:9.0-jdk8-openjdk"
	return fmt.Sprintf(`FROM %s
COPY %s /usr/local/tomcat/webapps/ROOT.war
CMD ["catalina.sh", "run"]
`, tomcatBaseImage, filepath.Base(config.War))
}

// createJavaBuildDockerfile 为Java构建工具创建Dockerfile
func createJavaBuildDockerfile(config *resource.BuildImageStep, buildType string) string {
	javaBaseImage := "maven:3.8.4-openjdk-8" // 默认Maven镜像
	if buildType == "gradle" {
		javaBaseImage = "gradle:7.3.0-jdk8"
	}

	var buildCmd string
	switch buildType {
	case "maven":
		buildCmd = "mvn clean package -DskipTests"
	case "gradle":
		buildCmd = "./gradlew build -x test"
	}

	return fmt.Sprintf(`FROM %s
WORKDIR /app
COPY . .
RUN %s
EXPOSE 8080
CMD ["java", "-jar", "target/*.jar"]
`, javaBaseImage, buildCmd)
}

// createPythonDockerfile 为Python项目创建Dockerfile
func createPythonDockerfile(config *resource.BuildImageStep) string {
	return `FROM python:3.9-slim
WORKDIR /app
COPY requirements.txt .
RUN pip install -r requirements.txt
COPY . .
EXPOSE 8000
CMD ["python", "app.py"]
`
}

// createNodeDockerfile 为Node.js项目创建Dockerfile
func createNodeDockerfile(config *resource.BuildImageStep) string {
	return `FROM node:16-alpine
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
EXPOSE 3000
CMD ["node", "index.js"]
`
}

// createPhpDockerfile 为PHP项目创建Dockerfile
func createPhpDockerfile(config *resource.BuildImageStep) string {
	return `FROM php:8.0-apache
COPY . /var/www/html/
EXPOSE 80
CMD ["apache2-foreground"]
`
}

// createGolangDockerfile 为Golang项目创建Dockerfile
func createGolangDockerfile(config *resource.BuildImageStep) string {
	return `FROM golang:1.19-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o main .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]
`
}

// createScalaDockerfile 为Scala项目创建Dockerfile
func createScalaDockerfile(config *resource.BuildImageStep) string {
	return `FROM hseeberger/scala-sbt:11.0.6_1.3.0_openjdk-8
WORKDIR /app
COPY . .
RUN sbt package
EXPOSE 8080
CMD ["sbt", "run"]
`
}

// renderDockerfileTemplate 渲染 Dockerfile 模板
func renderDockerfileTemplate(tmplStr string, config *resource.BuildImageStep) string {
	// 创建一个简单的模板数据上下文
	data := map[string]interface{}{
		"From":    config.ImageFrom,
		"Jar":     filepath.Base(config.Jar),
		"War":     filepath.Base(config.War),
		"AppName": config.AppName,
	}

	tmpl, err := template.New("dockerfile").Parse(tmplStr)
	if err != nil {
		// 如果解析失败，直接返回原字符串
		return tmplStr
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		// 如果执行失败，直接返回原字符串
		return tmplStr
	}

	return buf.String()
}

// renderDockerignoreTemplate 渲染.dockerignore 模板
func renderDockerignoreTemplate(tmplStr string, config *resource.BuildImageStep) string {
	// 创建一个简单的模板数据上下文
	data := map[string]interface{}{
		"Jar":   filepath.Base(config.Jar),
		"War":   filepath.Base(config.War),
		"Files": make(map[string]string), // 可以扩展支持多文件
	}

	tmpl, err := template.New("dockerignore").Parse(tmplStr)
	if err != nil {
		// 如果解析失败，直接返回原字符串
		return tmplStr
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		// 如果执行失败，直接返回原字符串
		return tmplStr
	}

	return buf.String()
}
