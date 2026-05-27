package proxy

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"
)

// NewHTTPClient 创建带代理的 HTTP 客户端。
// proxyEndpoint 为空时返回使用默认 transport 的客户端。
func NewHTTPClient(proxyEndpoint string, timeout time.Duration) *http.Client {
	if proxyEndpoint == "" {
		return &http.Client{Timeout: timeout}
	}
	proxyURL, err := url.Parse(proxyEndpoint)
	if err != nil {
		return &http.Client{Timeout: timeout}
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}
}
