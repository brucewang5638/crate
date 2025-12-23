package config

import (
	"os"
	"os/exec"
	"strings"
)

// detectDist 优先使用 rpm --eval 获取系统定义的 dist 后缀
func detectDist() string {
	// 1. 尝试直接询问 rpm (最准确，跟 rpmbuild 行为一致)
	cmd := exec.Command("rpm", "--eval", "%{dist}")
	if out, err := cmd.CombinedOutput(); err == nil {
		dist := strings.TrimSpace(string(out))
		// 确保输出不是原样返回 (说明没定义) 且非空
		if dist != "" && dist != "%{dist}" {
			return strings.TrimPrefix(dist, ".") // 去掉前面的点, .el7 -> el7
		}
	}

	// 2. 如果失败，回退到读取 /etc/os-release
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "el7"
	}
	content := strings.ToLower(string(data))

	// 1) 提取 VERSION_ID (比如 V10 或 7)
	var versionID string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "VERSION_ID=") {
			// 去掉引号 VERSION_ID="V10" -> V10
			versionID = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), `"`)
			break
		}
	}
	versionID = strings.ToLower(versionID)
	if versionID == "" {
		versionID = "7"
	}

	// 2) 映射前缀
	var prefix string
	if strings.Contains(content, "kylin") {
		prefix = "ky"
	} else if strings.Contains(content, "centos") {
		prefix = "el"
	} else {
		prefix = "el"
	}

	// 3) 拼接
	return prefix + versionID
}
