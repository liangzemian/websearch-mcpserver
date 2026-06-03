//go:build !windows

package proxy

func init() {
	// 非 Windows 平台仅依赖环境变量检测系统代理（已在 sysproxy.go 中实现）。
	// systemProxyDetector 保持 nil，detectSystemProxy() 会跳过 OS 检测。
}
