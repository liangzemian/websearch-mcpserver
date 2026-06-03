package proxy

import (
	"net/url"
	"os"
	"strings"
)

// systemProxyDetector 平台相关的系统代理检测函数。
// 在 Windows 上由 sysproxy_windows.go 的 init() 注入。
// 非 Windows 平台保持 nil（仅依赖环境变量）。
var systemProxyDetector func() string

// DetectSystemProxy 自动检测系统代理。
// 优先检查环境变量（HTTP_PROXY / HTTPS_PROXY / ALL_PROXY），
// 然后读取操作系统级代理设置（Windows 注册表 / WinHTTP）。
// 返回代理端点 URL（如 "http://127.0.0.1:7897"），未检测到则返回空字符串。
func DetectSystemProxy() string {
	// 1. 环境变量（优先级最高）
	if ep := proxyFromEnv(); ep != "" {
		return ep
	}
	// 2. 操作系统代理设置（平台相关，由各平台 init() 注入）
	if systemProxyDetector != nil {
		if ep := systemProxyDetector(); ep != "" {
			return ep
		}
	}
	return ""
}

// proxyFromEnv 从 HTTP_PROXY / HTTPS_PROXY / ALL_PROXY 环境变量读取代理。
func proxyFromEnv() string {
	for _, key := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			if u, err := url.Parse(v); err == nil && (u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "socks5") {
				return v
			}
		}
	}
	return ""
}
