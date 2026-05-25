//go:build windows

package daemon

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

// IsRunning 检测进程是否存活。
func IsRunning(pid int) bool {
	// 方法1: OpenProcess（可能因权限不足失败）
	handle, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err == nil {
		defer syscall.CloseHandle(handle)
		var exitCode uint32
		if err := syscall.GetExitCodeProcess(handle, &exitCode); err == nil {
			return exitCode == 259 // STILL_ACTIVE
		}
	}

	// 方法2: tasklist 兜底（不受权限限制）
	out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH", "/FO", "CSV").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), fmt.Sprintf(`"%d"`, pid))
}
