package ddg

import (
	"net/http"
	"time"

	"websearch/pkg/antirobot"
	"websearch/pkg/proxy"
)

// DuckDuckGoOpts DuckDuckGo 搜索配置。
type DuckDuckGoOpts struct {
	Enabled      bool
	Blocked      []string             // 屏蔽域名
	PerSec       int                  // 每秒限流，默认 2
	PerMin       int                  // 每分钟限流，默认 40
	ProxyResolve proxy.ProxyResolver  // 代理端点动态解析函数（每次请求实时获取）
}

// NewDuckDuckGo 创建 DuckDuckGo 引擎（需代理访问）。
func NewDuckDuckGo(opts DuckDuckGoOpts) antirobot.Engine {
	perSec, perMin := opts.PerSec, opts.PerMin
	if perSec <= 0 {
		perSec = 2
	}
	if perMin <= 0 {
		perMin = 40
	}
	e := &ddgEngine{
		opts:    opts,
		limiter: antirobot.NewRateLimiter(perSec, perMin),
	}
	e.rotateSession()
	return e
}

func (o DuckDuckGoOpts) newHTTPClient() *http.Client {
	return proxy.NewDynamicHTTPClient(o.ProxyResolve, 15*time.Second)
}
