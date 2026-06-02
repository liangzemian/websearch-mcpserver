package google

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"

	"websearch/pkg/antirobot"
)

// GoogleOpts Google 搜索配置。
type GoogleOpts struct {
	Enabled       bool
	Blocked       []string // 屏蔽域名
	PerSec        int      // 每秒限流，默认 1
	PerMin        int      // 每分钟限流，默认 10
	ProxyEndpoint string   // 代理端点（必需，Google 在国内需代理访问）
}

// NewGoogle 创建 Google 搜索引擎。
func NewGoogle(opts GoogleOpts) antirobot.Engine {
	perSec, perMin := opts.PerSec, opts.PerMin
	if perSec <= 0 {
		perSec = 3
	}
	if perMin <= 0 {
		perMin = 30
	}
	e := &googleEngine{
		opts:    opts,
		limiter: antirobot.NewRateLimiter(perSec, perMin),
	}
	e.rotateSession()
	return e
}

func (o GoogleOpts) newHTTPClient() *http.Client {
	if o.ProxyEndpoint == "" {
		return &http.Client{Timeout: 15 * time.Second}
	}
	proxyURL, err := url.Parse(o.ProxyEndpoint)
	if err != nil {
		return &http.Client{Timeout: 15 * time.Second}
	}
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}
}
