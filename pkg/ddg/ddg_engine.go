package ddg

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"websearch/pkg/antirobot"

	"github.com/PuerkitoBio/goquery"
)

// ──────────────────────────────────────────────────────────────────────────────
// DuckDuckGo 通用网页搜索引擎（HTML POST，需代理访问）
// ──────────────────────────────────────────────────────────────────────────────

type ddgEngine struct {
	opts    DuckDuckGoOpts
	limiter *antirobot.RateLimiter

	mu          sync.Mutex
	client      *http.Client
	ua          string
	reqCount    int
	backoff     time.Duration
	consecFails int
}

var ddgUAs = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
}

var ddgAcceptVariants = []string{
	"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
}

var ddgLangVariants = []string{
	"en-US,en;q=0.9",
	"en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7",
	"zh-CN,zh;q=0.9,en;q=0.8",
}

var ddgTimeRangeMap = map[antirobot.TimeRange]string{
	antirobot.TimeRangeDay:   "d",
	antirobot.TimeRangeWeek:  "w",
	antirobot.TimeRangeMonth: "m",
	antirobot.TimeRangeYear:  "y",
}

const (
	ddgBaseDelay       = 800 * time.Millisecond
	ddgJitter          = 1 * time.Second
	ddgMaxBackoff      = 90 * time.Second
	ddgSessionLifetime = 25
	ddgResultsPerPage  = 30
)

// ── 接口实现 ──

func (e *ddgEngine) Name() string                    { return "duckduckgo" }
func (e *ddgEngine) Region() antirobot.NetworkRegion { return antirobot.RegionInternational }

func (e *ddgEngine) Search(query string, page int, timeRange antirobot.TimeRange) (*antirobot.SearchResponse, error) {
	if !e.limiter.Allow() {
		return &antirobot.SearchResponse{Engine: "duckduckgo", Results: []antirobot.Result{}}, nil
	}

	e.preDelay()

	req, err := e.buildRequest(query, page, timeRange)
	if err != nil {
		return nil, err
	}
	e.setHeaders(req)

	resp, err := e.client.Do(req)
	if err != nil {
		e.recordFail()
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		e.recordFail()
		return nil, err
	}

	if resp.StatusCode != 200 {
		e.recordFail()
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	html := string(body)
	if !strings.Contains(html, "result") {
		e.recordFail()
		return nil, fmt.Errorf("no results container found, possibly blocked")
	}

	e.recordSuccess()
	results := e.parseResults(html)

	if len(e.opts.Blocked) > 0 {
		results = e.filterBlocked(results)
	}

	e.rotateSessionIfNeeded()

	return &antirobot.SearchResponse{Engine: "duckduckgo", Results: results}, nil
}

// ── 请求构建 ──

func (e *ddgEngine) buildRequest(query string, page int, timeRange antirobot.TimeRange) (*http.Request, error) {
	form := url.Values{}
	form.Set("q", e.applyBlocked(query))
	form.Set("ia", "web")

	// 分页：DDG HTML 版使用 s 参数（0-based offset）
	if page > 1 {
		form.Set("s", fmt.Sprintf("%d", (page-1)*ddgResultsPerPage))
		form.Set("dc", fmt.Sprintf("%d", (page-1)*ddgResultsPerPage))
	}

	// 时间范围
	if tr, ok := ddgTimeRangeMap[timeRange]; ok {
		form.Set("df", tr)
	}

	req, err := http.NewRequest("POST", "https://html.duckduckgo.com/html/",
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

func (e *ddgEngine) setHeaders(req *http.Request) {
	e.mu.Lock()
	ua := e.ua
	e.mu.Unlock()
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", ddgAcceptVariants[rand.Intn(len(ddgAcceptVariants))])
	req.Header.Set("Accept-Language", ddgLangVariants[rand.Intn(len(ddgLangVariants))])
	req.Header.Set("DNT", "1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Referer", "https://html.duckduckgo.com/")
	req.Header.Set("Origin", "https://html.duckduckgo.com")
}

// ── 反爬防御 ──

func (e *ddgEngine) preDelay() {
	e.mu.Lock()
	bo := e.backoff
	e.mu.Unlock()
	delay := ddgBaseDelay + time.Duration(rand.Int63n(int64(ddgJitter)))
	if bo > 0 {
		delay += bo
	}
	time.Sleep(delay)
}

func (e *ddgEngine) recordSuccess() {
	e.mu.Lock()
	e.backoff = 0
	e.consecFails = 0
	e.mu.Unlock()
}

func (e *ddgEngine) recordFail() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.consecFails++
	bo := ddgBaseDelay * time.Duration(1<<uint(e.consecFails))
	if bo > ddgMaxBackoff {
		bo = ddgMaxBackoff
	}
	e.backoff = bo
}

func (e *ddgEngine) rotateSessionIfNeeded() {
	e.mu.Lock()
	e.reqCount++
	rotate := e.reqCount >= ddgSessionLifetime
	e.mu.Unlock()
	if rotate {
		e.rotateSession()
	}
}

func (e *ddgEngine) rotateSession() {
	e.client = e.opts.newHTTPClient()
	e.ua = ddgUAs[rand.Intn(len(ddgUAs))]
	e.reqCount = 0
}

// ── HTML 解析 ──

func (e *ddgEngine) parseResults(htmlText string) []antirobot.Result {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return nil
	}

	var results []antirobot.Result

	doc.Find("div.result").Each(func(_ int, sel *goquery.Selection) {
		// 跳过广告
		if sel.HasClass("result--ad") {
			return
		}

		link := sel.Find("a.result__a").First()
		if link.Length() == 0 {
			return
		}

		title := strings.TrimSpace(link.Text())
		href, _ := link.Attr("href")
		if href == "" || title == "" {
			return
		}

		// DDG 的链接可能是相对路径或跳转链接，提取真实 URL
		href = decodeDDGURL(href)

		// 摘要
		content := ""
		if snippet := sel.Find(".result__snippet").First(); snippet.Length() > 0 {
			content = antirobot.CollapseSpace(strings.TrimSpace(snippet.Text()))
		}

		// 来源
		source := ""
		if src := sel.Find(".result__url").First(); src.Length() > 0 {
			source = strings.TrimSpace(src.Text())
		}

		_ = source // source 可用于未来扩展

		results = append(results, antirobot.Result{
			Type:    antirobot.ResultWeb,
			Title:   title,
			URL:     href,
			Content: content,
			Engine:  "duckduckgo",
		})
	})

	return results
}

// decodeDDGURL 处理 DuckDuckGo 的跳转链接，提取真实 URL。
func decodeDDGURL(href string) string {
	// 处理 //duckduckgo.com/l/?uddg=... 格式
	if strings.HasPrefix(href, "//") {
		href = "https:" + href
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return href
	}
	if strings.HasSuffix(parsed.Hostname(), "duckduckgo.com") && parsed.Path == "/l/" {
		realURL := parsed.Query().Get("uddg")
		if realURL != "" {
			return realURL
		}
	}
	return href
}

// ── 站点屏蔽 ──

func (e *ddgEngine) applyBlocked(query string) string {
	if len(e.opts.Blocked) > 5 {
		return query
	}
	var sb strings.Builder
	sb.WriteString(query)
	for _, d := range e.opts.Blocked {
		sb.WriteString(" -site:")
		sb.WriteString(d)
	}
	return sb.String()
}

func (e *ddgEngine) filterBlocked(results []antirobot.Result) []antirobot.Result {
	blocked := make(map[string]struct{}, len(e.opts.Blocked))
	for _, d := range e.opts.Blocked {
		blocked[strings.ToLower(d)] = struct{}{}
	}
	filtered := make([]antirobot.Result, 0, len(results))
	for _, r := range results {
		host := extractHost(r.URL)
		hit := false
		for d := range blocked {
			if host == d || strings.HasSuffix(host, "."+d) {
				hit = true
				break
			}
		}
		if !hit {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func extractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	return strings.TrimPrefix(host, "www.")
}
