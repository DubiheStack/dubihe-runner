package license

import (
	"fmt"
	"time"
)

const (
	UNLIMITED = "unlimited"
)

// LicenseInfo 包含许可证信息，与Java版本保持一致
type LicenseInfo struct {
	OsName      string            `json:"osName"`
	CpuInfo     string            `json:"cpuInfo"`
	DiskInfo    string            `json:"diskInfo"`
	MacAddress  string            `json:"macAddress"`
	StartDate   string            `json:"startDate"`
	LimitDays   string            `json:"limitDays"`
	CompanyName string            `json:"companyName"`
	Limitations map[string]string `json:"limitations"`
}

// NewLicenseInfo 创建新的LicenseInfo实例
func NewLicenseInfo() *LicenseInfo {
	return &LicenseInfo{
		Limitations: make(map[string]string),
	}
}

// Set 设置许可证属性
func (li *LicenseInfo) Set(key, value string) {
	switch key {
	case "osName":
		li.OsName = value
	case "cpuInfo":
		li.CpuInfo = value
	case "diskInfo":
		li.DiskInfo = value
	case "macAddress":
		li.MacAddress = value
	case "startDate":
		li.StartDate = value
	case "limitDays":
		li.LimitDays = value
	case "companyName":
		li.CompanyName = value
	default:
		li.Limitations[key] = value
	}
}

// Get 获取许可证属性
func (li *LicenseInfo) Get(key string) string {
	switch key {
	case "osName":
		return li.OsName
	case "cpuInfo":
		return li.CpuInfo
	case "diskInfo":
		return li.DiskInfo
	case "macAddress":
		return li.MacAddress
	case "startDate":
		return li.StartDate
	case "limitDays":
		return li.LimitDays
	case "companyName":
		return li.CompanyName
	default:
		return li.Limitations[key]
	}
}

// IsSameServer 检查是否为同一台服务器
func (li *LicenseInfo) IsSameServer(cpuInfo, diskInfo, macAddress string) bool {
	// 如果license中的任何字段为空，则跳过验证
	if li.CpuInfo == "" || li.DiskInfo == "" || li.MacAddress == "" {
		return true
	}

	return li.CpuInfo == cpuInfo && li.DiskInfo == diskInfo && li.MacAddress == macAddress
}

// IsExpired 检查许可证是否过期
func (li *LicenseInfo) IsExpired() (bool, error) {
	// 如果许可证信息为空，返回false（没有许可证信息不意味着过期）
	if li.LimitDays == "" && li.StartDate == "" {
		return false, nil
	}

	if li.LimitDays == UNLIMITED {
		return false, nil
	}

	if li.StartDate == "" || li.LimitDays == "" {
		return true, fmt.Errorf("missing startDate or limitDays")
	}

	// 解析开始日期
	layout := "2006-01-02"
	startDate, err := time.Parse(layout, li.StartDate)
	if err != nil {
		return true, fmt.Errorf("failed to parse startDate: %v", err)
	}

	// 解析限制天数
	var limitDays int
	_, err = fmt.Sscanf(li.LimitDays, "%d", &limitDays)
	if err != nil {
		return true, fmt.Errorf("failed to parse limitDays: %v", err)
	}

	// 计算结束日期
	endDate := startDate.AddDate(0, 0, limitDays)

	// 检查是否过期
	return time.Now().After(endDate), nil
}

// EndTime 获取许可证结束时间
func (li *LicenseInfo) EndTime() (string, error) {
	// 如果许可证信息为空，返回unlimited
	if li.LimitDays == "" && li.StartDate == "" {
		return "unlimited", nil
	}

	if li.LimitDays == UNLIMITED {
		return "unlimited", nil
	}

	if li.StartDate == "" || li.LimitDays == "" {
		return "", fmt.Errorf("missing startDate or limitDays")
	}

	// 解析开始日期
	layout := "2006-01-02"
	startDate, err := time.Parse(layout, li.StartDate)
	if err != nil {
		return "", fmt.Errorf("failed to parse startDate: %v", err)
	}

	// 解析限制天数
	var limitDays int
	_, err = fmt.Sscanf(li.LimitDays, "%d", &limitDays)
	if err != nil {
		return "", fmt.Errorf("failed to parse limitDays: %v", err)
	}

	// 计算结束日期
	endDate := startDate.AddDate(0, 0, limitDays)

	return endDate.Format(layout), nil
}

// GetOsName 获取操作系统名称
func (li *LicenseInfo) GetOsName() string {
	return li.OsName
}

// GetCpuInfo 获取CPU信息
func (li *LicenseInfo) GetCpuInfo() string {
	return li.CpuInfo
}

// GetDiskInfo 获取磁盘信息
func (li *LicenseInfo) GetDiskInfo() string {
	return li.DiskInfo
}

// GetMacAddress 获取MAC地址
func (li *LicenseInfo) GetMacAddress() string {
	return li.MacAddress
}

// GetStartDate 获取开始授权时间
func (li *LicenseInfo) GetStartDate() string {
	return li.StartDate
}

// GetLimitDays 获取系统使用天数限制
func (li *LicenseInfo) GetLimitDays() string {
	return li.LimitDays
}

// GetCompanyName 获取公司名称
func (li *LicenseInfo) GetCompanyName() string {
	return li.CompanyName
}

// SetOsName 设置操作系统名称
func (li *LicenseInfo) SetOsName(osName string) {
	li.OsName = osName
}

// SetCpuInfo 设置CPU信息
func (li *LicenseInfo) SetCpuInfo(cpuInfo string) {
	li.CpuInfo = cpuInfo
}

// SetDiskInfo 设置磁盘信息
func (li *LicenseInfo) SetDiskInfo(diskInfo string) {
	li.DiskInfo = diskInfo
}

// SetMacAddress 设置MAC地址
func (li *LicenseInfo) SetMacAddress(macAddress string) {
	li.MacAddress = macAddress
}

// SetStartDate 设置开始授权时间
func (li *LicenseInfo) SetStartDate(startDate string) {
	li.StartDate = startDate
}

// SetLimitDays 设置系统使用天数限制
func (li *LicenseInfo) SetLimitDays(limitDays string) {
	li.LimitDays = limitDays
}

// SetCompanyName 设置公司名称
func (li *LicenseInfo) SetCompanyName(companyName string) {
	li.CompanyName = companyName
}
