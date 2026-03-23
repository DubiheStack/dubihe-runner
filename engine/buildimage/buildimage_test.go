package buildimage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/DubiheStack/dubihe-runner/resource"
)

func TestFixImage_CustomImage(t *testing.T) {
	ctx := context.Background()
	config := &resource.BuildImageStep{
		UseCustomImage: true,
		ImageFrom:      "openjdk:8-jre-alpine",
		ImageTo:        "harbor.example.com/library/test-app",
		PackageVersion: "v1.0.0",
	}

	err := FixImage(ctx, config)
	if err != nil {
		t.Fatalf("FixImage failed: %v", err)
	}

	// 验证是否添加了时间戳 tag
	if config.ImageTo == "harbor.example.com/library/test-app" {
		t.Error("Expected image to have timestamp tag")
	}

	t.Logf("ImageTo after FixImage: %s", config.ImageTo)
}

func TestCreateDockerfile_Jar(t *testing.T) {
	workDir := t.TempDir()
	config := &resource.BuildImageStep{
		Jar:       "./target/app-1.0.0.jar",
		ImageFrom: "openjdk:8-jre-alpine",
		ImageTo:   "harbor.example.com/library/test-app:v1.0.0",
	}

	err := CreateDockerfile(config, workDir, "jar")
	if err != nil {
		t.Fatalf("CreateDockerfile failed: %v", err)
	}

	// 验证 Dockerfile 是否创建
	dockerfilePath := filepath.Join(workDir, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		t.Error("Dockerfile was not created")
	}

	// 验证.dockerignore 是否创建
	dockerignorePath := filepath.Join(workDir, ".dockerignore")
	if _, err := os.Stat(dockerignorePath); os.IsNotExist(err) {
		t.Error(".dockerignore was not created")
	}

	// 读取并验证 Dockerfile 内容
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Fatalf("Failed to read Dockerfile: %v", err)
	}

	expectedContent := "FROM openjdk:8-jre-alpine"
	if string(content) == "" {
		t.Error("Dockerfile is empty")
	}

	if len(string(content)) > 0 && string(content)[:len(expectedContent)] != expectedContent {
		t.Errorf("Dockerfile doesn't start with '%s', got: %s", expectedContent, string(content))
	}

	t.Logf("Dockerfile content:\n%s", string(content))
}

func TestBuildDockerImage_EmptyImageTo(t *testing.T) {
	ctx := context.Background()
	config := &resource.BuildImageStep{
		ImageTo: "", // 空的 ImageTo
	}

	workDir := t.TempDir()
	err := BuildDockerImage(ctx, config, workDir)
	if err == nil {
		t.Error("Expected error for empty ImageTo")
	}

	expectedErr := "image target is empty"
	if err != nil && err.Error() != expectedErr+", please check image configuration" {
		t.Errorf("Expected error '%s', got '%v'", expectedErr, err)
	}
}

func TestRenderDockerfileTemplate(t *testing.T) {
	config := &resource.BuildImageStep{
		ImageFrom: "openjdk:11-jre-slim",
		Jar:       "./target/myapp.jar",
		AppName:   "myapp",
	}

	template := `FROM {{.From}}
COPY {{.Jar}} /app.jar
EXPOSE 8080
ENTRYPOINT ["java", "-jar", "/app.jar"]`

	result := renderDockerfileTemplate(template, config)

	expectedStart := "FROM openjdk:11-jre-slim"
	if len(result) > 0 && result[:len(expectedStart)] != expectedStart {
		t.Errorf("Expected template to start with '%s', got '%s'", expectedStart, result)
	}

	if len(result) == 0 {
		t.Error("Rendered template is empty")
	}

	t.Logf("Rendered template:\n%s", result)
}

func TestRenderDockerignoreTemplate(t *testing.T) {
	config := &resource.BuildImageStep{
		Jar: "./target/app.jar",
	}

	template := `*
!{{.Jar}}`

	result := renderDockerignoreTemplate(template, config)

	if len(result) == 0 {
		t.Error("Rendered dockerignore is empty")
	}

	t.Logf("Rendered dockerignore:\n%s", result)
}

func TestSendBuildResult_MockServer(t *testing.T) {
	// 这个测试需要一个 mock HTTP server
	// 实际使用时可以集成测试
	t.Skip("Skipping integration test - requires mock HTTP server")
}
