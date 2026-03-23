package license

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

const (
	ENCRYPT_LENGTH = 4
)

var (
	instance   *LicenseChecker
	instanceMu sync.Mutex
)

// LicenseChecker 许可证检查器
type LicenseChecker struct {
	licenseInfo  *LicenseInfo
	licensePaths []string
}

// GetInstance 获取LicenseChecker单例实例
func GetInstance() *LicenseChecker {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if instance == nil {
		instance = &LicenseChecker{
			licensePaths: []string{
				"./distribution/license/license",
				"./config/license",
				"./license",
			},
		}
		instance.initLicense()
	}

	return instance
}

// GetInstanceWithConfigPath 使用指定的配置路径创建LicenseChecker实例
func GetInstanceWithConfigPath(configLicensePath string) *LicenseChecker {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if instance == nil {
		instance = &LicenseChecker{
			licensePaths: []string{
				configLicensePath,
				"./distribution/license/license",
				"./config/license",
				"./license",
			},
		}
		instance.initLicense()
	}

	return instance
}

// initLicense 初始化许可证
func (lc *LicenseChecker) initLicense() {
	licenseConfig := os.Getenv("license.config")
	if licenseConfig != "" {
		licenseConfig = licenseConfig + "/license"
		logrus.Infof("load license from env,path: %s", licenseConfig)
		lc.licensePaths = append(lc.licensePaths, licenseConfig)
	}

	logrus.Infof("Checking for license files in paths: %v", lc.licensePaths)

	foundLicense := false
	for _, licensePath := range lc.licensePaths {
		logrus.Infof("Checking license path: %s", licensePath)
		if lc.fileExists(licensePath) {
			logrus.Infof("Found license file: %s", licensePath)
			err := lc.loadLicense(licensePath)
			if err != nil {
				logrus.Errorf("load license fail, licensePath is: %s, error: %v", licensePath, err)
				continue
			}
			foundLicense = true
			logrus.Infof("Successfully loaded license from: %s", licensePath)
			break
		} else {
			logrus.Infof("License file not found: %s", licensePath)
		}
	}

	if !foundLicense {
		logrus.Warnf("couldn't find any license from paths: %v, Running without license", lc.licensePaths)
		// 不再强制退出，而是创建一个空的LicenseInfo对象
		lc.licenseInfo = NewLicenseInfo()
	}
}

// fileExists 检查文件是否存在
func (lc *LicenseChecker) fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// loadLicense 加载许可证文件
func (lc *LicenseChecker) loadLicense(licensePath string) error {
	logrus.Infof("Loading license from path: %s", licensePath)

	content, err := os.ReadFile(licensePath)
	if err != nil {
		return fmt.Errorf("failed to read license file: %v", err)
	}

	logrus.Infof("License file size: %d bytes", len(content))

	// 尝试作为文本文件读取一行，判断是否为明文license
	firstLine := ""
	if len(content) > 0 {
		scanner := bufio.NewScanner(strings.NewReader(string(content)))
		if scanner.Scan() {
			firstLine = scanner.Text()
		}
	}
	logrus.Infof("First line of license file: %q", firstLine)

	// 默认将整个内容作为许可证内容
	licenseContent := content
	var privateKey []byte

	// 尝试检测是否有加密头部信息
	if len(content) > ENCRYPT_LENGTH {
		// 尝试读取头部长度信息
		lengthBytes := content[:ENCRYPT_LENGTH]
		contentLength := int(binary.BigEndian.Uint32(lengthBytes))

		logrus.Infof("Content length from header: %d bytes", contentLength)

		// 检查总信息长度(排除长度信息)
		totalLength := len(content) - ENCRYPT_LENGTH
		privateKeyStartIndex := contentLength + ENCRYPT_LENGTH

		// 检查索引是否有效以及内容长度是否合理
		if privateKeyStartIndex <= len(content) && contentLength <= totalLength && contentLength > 0 {
			logrus.Infof("Detected encrypted license format")
			contentStartIndex := ENCRYPT_LENGTH

			logrus.Infof("Content start index: %d, Private key start index: %d", contentStartIndex, privateKeyStartIndex)

			licenseContent = content[contentStartIndex:privateKeyStartIndex]
			privateKey = content[privateKeyStartIndex:]

			logrus.Infof("License content size: %d bytes, Private key size: %d bytes", len(licenseContent), len(privateKey))
		} else {
			logrus.Infof("Not an encrypted license or invalid format, treating as plain text")
		}
	} else {
		// 文件太小，不可能包含加密头部
		logrus.Infof("File too small for encryption header (%d < %d), treating as plain text", len(content), ENCRYPT_LENGTH)
	}

	// 尝试解密许可证内容
	var decryptedContent []byte
	if len(privateKey) > 0 && len(licenseContent) > 0 {
		privateKeyStr := string(privateKey)
		logrus.Infof("Private key string length: %d", len(privateKeyStr))

		// 尝试解密
		decryptedContent, err = decryptBySegment(licenseContent, privateKeyStr)
		if err != nil {
			logrus.Warnf("Failed to decrypt license content: %v, trying to parse as plain text", err)
			// 如果解密失败，将原始内容视为明文
			decryptedContent = licenseContent
		} else {
			logrus.Info("Successfully decrypted license content")
		}
	} else {
		// 没有私钥，将内容视为明文
		logrus.Info("No private key found or license content is empty, treating content as plain text")
		decryptedContent = licenseContent
	}

	logrus.Infof("Decrypted content size: %d bytes", len(decryptedContent))
	if len(decryptedContent) < 1000 { // 只记录小内容以避免日志过大
		logrus.Debugf("Decrypted content: %q", string(decryptedContent))
	}

	// 解析属性
	lc.licenseInfo = NewLicenseInfo()

	// 如果解密后的内容仍然不可读，尝试直接解析原始内容
	contentToParse := decryptedContent
	if len(decryptedContent) > 0 {
		// 检查解密后的内容是否可读
		isReadable := true
		for _, b := range decryptedContent {
			if b < 32 && b != 9 && b != 10 && b != 13 { // 允许制表符、换行符和回车符
				isReadable = false
				break
			}
		}

		if !isReadable {
			logrus.Warnf("Decrypted content is not readable, trying to parse original content")
			contentToParse = licenseContent
		}
	}

	lines := strings.Split(string(contentToParse), "\n")

	logrus.Infof("Number of lines in content to parse: %d", len(lines))

	validLines := 0
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		logrus.Debugf("Processing line %d: %q", i, line)

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			logrus.Debugf("Setting key='%s', value='%s'", key, value)
			lc.licenseInfo.Set(key, value)
			validLines++
		} else {
			logrus.Warnf("Invalid line format: %q", line)
		}
	}

	logrus.Infof("Parsed %d valid lines", validLines)

	// 记录加载的许可证信息
	if lc.licenseInfo != nil {
		logrus.Infof("License info loaded successfully")
		logrus.Debugf("CPU Info: '%s'", lc.licenseInfo.GetCpuInfo())
		logrus.Debugf("Disk Info: '%s'", lc.licenseInfo.GetDiskInfo())
		logrus.Debugf("MAC Address: '%s'", lc.licenseInfo.GetMacAddress())
		logrus.Debugf("Start Date: '%s'", lc.licenseInfo.GetStartDate())
		logrus.Debugf("Limit Days: '%s'", lc.licenseInfo.GetLimitDays())
		logrus.Debugf("Company Name: '%s'", lc.licenseInfo.GetCompanyName())
	}

	return nil
}

const (
	MAX_DECRYPT_BLOCK = 128
)

// decryptBySegment 分段解密
func decryptBySegment(encryptedData []byte, key string) ([]byte, error) {
	// 根据Java端的实现，私钥是Base64编码的，需要先解码
	decodedKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 private key: %v", err)
	}
	
	// 解析PKCS8格式私钥（根据Java端LicenseServiceImpl的实现）
	privateKey, err := x509.ParsePKCS8PrivateKey(decodedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PKCS8 private key: %v", err)
	}
	
	// 类型断言为RSA私钥
	rsaPrivateKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA private key")
	}

	// 分段解密，参考Java实现
	inputLen := len(encryptedData)
	var result []byte
	offset := 0

	for inputLen-offset > 0 {
		var blockSize int
		if inputLen-offset > MAX_DECRYPT_BLOCK {
			blockSize = MAX_DECRYPT_BLOCK
		} else {
			blockSize = inputLen - offset
		}

		// 确保不会越界
		if offset+blockSize > inputLen {
			blockSize = inputLen - offset
		}

		// 解密当前块
		blockData := encryptedData[offset : offset+blockSize]
		decryptedBlock, err := rsa.DecryptPKCS1v15(rand.Reader, rsaPrivateKey, blockData)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt block at offset %d: %v", offset, err)
		}

		// 添加到结果中
		result = append(result, decryptedBlock...)

		// 移动到下一个块
		offset += MAX_DECRYPT_BLOCK
	}

	return result, nil
}

// GetLicenseInfo 获取许可证信息
func (lc *LicenseChecker) GetLicenseInfo() *LicenseInfo {
	return lc.licenseInfo
}

// IsSameServer 检查是否为同一台服务器
func IsSameServer(cpuInfo, diskInfo, macAddress string) bool {
	checker := GetInstance()
	return checker.licenseInfo.IsSameServer(cpuInfo, diskInfo, macAddress)
}

// IsSameServerAuto 自动检查是否为同一台服务器
func IsSameServerAuto() bool {
	serverInfo, err := GatherServerInfo()
	if err != nil {
		logrus.Errorf("Failed to gather server info: %v", err)
		return false
	}

	return IsSameServer(serverInfo.CpuInfo, serverInfo.DiskInfo, serverInfo.MacAddress)
}

// IsExpired 检查许可证是否过期
func IsExpired() bool {
	checker := GetInstance()
	expired, err := checker.licenseInfo.IsExpired()
	if err != nil {
		logrus.Errorf("Failed to check license expiration: %v", err)
		return true
	}
	return expired
}

// GetEndTime 获取许可证结束时间
func GetEndTime() (string, error) {
	checker := GetInstance()
	return checker.licenseInfo.EndTime()
}

// ServerInfo 服务器信息
type ServerInfo struct {
	OsName     string
	CpuInfo    string
	DiskInfo   string
	MacAddress string
}

// GatherServerInfo 收集服务器信息
func GatherServerInfo() (*ServerInfo, error) {
	osName := runtime.GOOS
	serverInfo := &ServerInfo{
		OsName: osName,
	}

	var err error
	if strings.HasPrefix(osName, "windows") {
		serverInfo.CpuInfo, err = getWindowsCpuInfo()
		if err != nil {
			return nil, err
		}

		serverInfo.DiskInfo, err = getWindowsDiskInfo()
		if err != nil {
			return nil, err
		}

		serverInfo.MacAddress, err = getWindowsMacAddress()
		if err != nil {
			return nil, err
		}
	} else if strings.HasPrefix(osName, "linux") {
		serverInfo.CpuInfo, err = getLinuxCpuInfo()
		if err != nil {
			return nil, err
		}

		serverInfo.DiskInfo, err = getLinuxDiskInfo()
		if err != nil {
			return nil, err
		}

		serverInfo.MacAddress, err = getLinuxMacAddress()
		if err != nil {
			return nil, err
		}
	} else if strings.HasPrefix(osName, "darwin") {
		serverInfo.CpuInfo, err = getDarwinCpuInfo()
		if err != nil {
			return nil, err
		}

		serverInfo.DiskInfo, err = getDarwinDiskInfo()
		if err != nil {
			return nil, err
		}

		serverInfo.MacAddress, err = getDarwinMacAddress()
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("unsupported OS: %s", osName)
	}

	return serverInfo, nil
}

// Linux系统信息获取
func getLinuxCpuInfo() (string, error) {
	// 尝试多种方法获取CPU信息
	methods := []string{
		"dmidecode -t 4 | grep ID",
		"cat /proc/cpuinfo | grep Serial | head -n 1",
		"cat /proc/cpuinfo | grep 'cpu cores' | head -n 1",
	}

	for _, method := range methods {
		cmd := exec.Command("/bin/sh", "-c", method)
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			result := string(output)
			// 处理不同的输出格式
			lines := strings.Split(result, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.Contains(line, ":") {
					parts := strings.Split(line, ":")
					if len(parts) > 1 {
						value := strings.TrimSpace(parts[1])
						if value != "" {
							return value, nil
						}
					}
				} else if strings.Contains(line, "\tID:") {
					parts := strings.Split(line, "\tID:")
					if len(parts) > 1 {
						value := strings.TrimSpace(parts[1])
						if value != "" {
							return value, nil
						}
					}
				} else if line != "" {
					return line, nil
				}
			}
		}
	}

	return "", fmt.Errorf("failed to get CPU info")
}

func getLinuxDiskInfo() (string, error) {
	// 尝试多种方法获取磁盘序列号
	methods := []string{
		"lsblk -d -o SERIAL | tail -n +2 | head -n 1",
		"udevadm info --query=all --name=/dev/sda | grep ID_SERIAL_SHORT",
		"hdparm -I /dev/sda | grep 'Serial Number'",
	}

	for _, method := range methods {
		cmd := exec.Command("/bin/sh", "-c", method)
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			result := string(output)
			// 处理不同的输出格式
			if strings.Contains(result, "=") {
				parts := strings.Split(result, "=")
				if len(parts) > 1 {
					value := strings.TrimSpace(parts[1])
					if value != "" {
						return value, nil
					}
				}
			} else if strings.Contains(result, ":") {
				parts := strings.Split(result, ":")
				if len(parts) > 1 {
					value := strings.TrimSpace(parts[1])
					if value != "" {
						return value, nil
					}
				}
			} else {
				value := strings.TrimSpace(result)
				if value != "" {
					return value, nil
				}
			}
		}
	}

	return "", fmt.Errorf("failed to get disk info")
}

func getLinuxMacAddress() (string, error) {
	// 尝试多种方法获取MAC地址
	methods := []string{
		"ip link show | grep ether | head -n 1",
		"ifconfig | grep ether | head -n 1",
		"cat /sys/class/net/*/address | head -n 1",
	}

	for _, method := range methods {
		cmd := exec.Command("/bin/sh", "-c", method)
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			result := string(output)
			// 提取MAC地址 (格式为 xx:xx:xx:xx:xx:xx)
			re := regexp.MustCompile(`([0-9a-fA-F]{2}:){5}[0-9a-fA-F]{2}`)
			matches := re.FindStringSubmatch(result)
			if len(matches) > 0 {
				return matches[0], nil
			}

			// 如果正则表达式没有匹配，尝试手动提取
			fields := strings.Fields(result)
			for _, field := range fields {
				if strings.Contains(field, ":") && len(field) == 17 {
					// 验证是否为有效的MAC地址格式
					parts := strings.Split(field, ":")
					if len(parts) == 6 {
						valid := true
						for _, part := range parts {
							if len(part) != 2 {
								valid = false
								break
							}
						}
						if valid {
							return field, nil
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("failed to get MAC address")
}

// Windows系统信息获取
func getWindowsCpuInfo() (string, error) {
	cmd := exec.Command("wmic", "cpu", "get", "ProcessorId")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	result := string(output)
	return removeTitle(result, "ProcessorId"), nil
}

func getWindowsDiskInfo() (string, error) {
	cmd := exec.Command("wmic", "diskdrive", "get", "SerialNumber")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	result := string(output)
	return removeTitle(result, "SerialNumber"), nil
}

func getWindowsMacAddress() (string, error) {
	cmd := exec.Command("wmic", "nicconfig", "get", "MACAddress")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	result := string(output)
	return removeTitle(result, "MACAddress"), nil
}

func removeTitle(result, title string) string {
	scanner := bufio.NewScanner(strings.NewReader(result))
	var values []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// 移除标题
		if strings.Contains(line, title) {
			line = strings.Replace(line, title, "", 1)
			line = strings.TrimSpace(line)
		}

		if line != "" {
			values = append(values, line)
		}
	}

	return strings.Join(values, ",")
}

// Darwin/macOS系统信息获取
func getDarwinCpuInfo() (string, error) {
	// 获取CPU序列号或相关信息
	cmd := exec.Command("system_profiler", "SPHardwareDataType")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	result := string(output)
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Serial Number") || strings.Contains(line, "序列号") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	// 如果找不到序列号，使用硬件UUID作为替代
	cmd = exec.Command("system_profiler", "SPHardwareDataType")
	output, err = cmd.Output()
	if err != nil {
		return "", err
	}

	lines = strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Hardware UUID") || strings.Contains(line, "硬件UUID") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	return "", fmt.Errorf("failed to get CPU info on darwin")
}

func getDarwinDiskInfo() (string, error) {
	// 获取磁盘序列号
	cmd := exec.Command("system_profiler", "SPStorageDataType")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Volume UUID") || strings.Contains(line, "卷UUID") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	// 备选方案：获取主磁盘信息
	cmd = exec.Command("diskutil", "info", "/")
	output, err = cmd.Output()
	if err != nil {
		return "", err
	}

	lines = strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Volume UUID") || strings.Contains(line, "卷UUID") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	return "", fmt.Errorf("failed to get disk info on darwin")
}

func getDarwinMacAddress() (string, error) {
	// 获取网络接口的MAC地址
	cmd := exec.Command("ifconfig", "en0")
	output, err := cmd.Output()
	if err != nil {
		// 尝试en1接口
		cmd = exec.Command("ifconfig", "en1")
		output, err = cmd.Output()
		if err != nil {
			return "", err
		}
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "ether") {
			fields := strings.Fields(line)
			for _, field := range fields {
				// 检查是否为MAC地址格式
				if len(field) == 17 && strings.Count(field, ":") == 5 {
					return field, nil
				}
			}
		}
	}

	return "", fmt.Errorf("failed to get MAC address on darwin")
}
