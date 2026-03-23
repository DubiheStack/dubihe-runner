// Package monitor 提供系统资源监控功能
package monitor

import (
	"fmt"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// ResourceUsage 表示系统资源使用情况
type ResourceUsage struct {
	CPU    string `json:"cpu"`     // CPU 使用率
	Mem    string `json:"mem"`     // 内存使用量(MB)
	Disk   string `json:"disk"`    // 磁盘使用量(MB)
	HostID string `json:"hostUid"` // 主机ID
}

// Collect 收集系统资源使用情况
func Collect(hostID string) (*ResourceUsage, error) {
	usage := &ResourceUsage{
		HostID: hostID,
	}

	// 收集 CPU 使用率
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get cpu percent: %w", err)
	}
	if len(cpuPercent) > 0 {
		usage.CPU = fmt.Sprintf("%.2f", cpuPercent[0])
	} else {
		usage.CPU = "0.00"
	}

	// 收集内存使用情况
	memStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory info: %w", err)
	}
	usage.Mem = fmt.Sprintf("%.2f", float64(memStat.Used)/1024/1024) // 转换为 MB

	// 收集磁盘使用情况
	diskStat, err := disk.Usage("/")
	if err != nil {
		// 如果根目录无法访问，尝试其他方式获取磁盘使用情况
		diskStat, err = disk.Usage(".")
		if err != nil {
			return nil, fmt.Errorf("failed to get disk usage: %w", err)
		}
	}
	usage.Disk = fmt.Sprintf("%.2f", float64(diskStat.Used)/1024/1024) // 转换为 MB

	return usage, nil
}

// GetGoRuntimeStats 获取 Go 运行时统计信息
func GetGoRuntimeStats() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	stats := make(map[string]interface{})
	stats["goroutines"] = runtime.NumGoroutine()
	stats["memory_allocated"] = fmt.Sprintf("%.2f MB", float64(m.Alloc)/1024/1024)
	stats["memory_sys"] = fmt.Sprintf("%.2f MB", float64(m.Sys)/1024/1024)
	stats["num_gc"] = m.NumGC

	return stats
}
