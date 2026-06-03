package academic

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"websearch/pkg/antirobot"
	"websearch/pkg/proxy"
)

// ──────────────────────────────────────────────────────────────────────────────
// arXiv 预印本搜索（海外优先）
// ──────────────────────────────────────────────────────────────────────────────

type arxivEngine struct {
	client  *http.Client
	limiter *antirobot.RateLimiter
}

// NewArxiv 创建 arXiv 引擎。client 为 nil 时使用带限流重试的默认客户端。
// arXiv 官方建议每秒不超过 1 次请求，超出返回 429。
func NewArxiv(_ antirobot.ArxivOpts, client *http.Client) antirobot.Engine {
	if client == nil {
		client = proxy.NewDynamicHTTPClient(nil, 15*time.Second)
	}
	return &arxivEngine{
		client:  client,
		limiter: antirobot.NewRateLimiter(1, 20), // 1/s, 20/min
	}
}

func (e *arxivEngine) Name() string                    { return "arxiv" }
func (e *arxivEngine) Region() antirobot.NetworkRegion { return antirobot.RegionChina }

func (e *arxivEngine) Search(query string, page int, timeRange antirobot.TimeRange) (*antirobot.SearchResponse, error) {
	offset := (page - 1) * 10
	if offset < 0 {
		offset = 0
	}

	// 限流等待：arXiv 官方建议不超过 1 req/s
	for !e.limiter.Allow() {
		time.Sleep(500 * time.Millisecond)
	}

	q := "all:" + query
	if since := antirobot.TimeRangeSince(timeRange); since != "" {
		q += " AND submittedDate:[" + since + " TO *]"
	}

	u := fmt.Sprintf("https://export.arxiv.org/api/query?search_query=%s&start=%d&max_results=10",
		url.QueryEscape(q), offset)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("arxiv request: %w", err)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("arxiv request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("arxiv HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return e.parse(body)
}

// ── XML 解析 ──

type arxivFeed struct {
	Entries []arxivEntry `xml:"entry"`
}

type arxivEntry struct {
	Title     string      `xml:"title"`
	ID        string      `xml:"id"`
	Summary   string      `xml:"summary"`
	Published string      `xml:"published"`
	Authors   []arxivAuthor `xml:"author"`
	Links     []arxivLink   `xml:"link"`
	DOI       string      `xml:"doi"`
	Category  []arxivCat  `xml:"category"`
	Comment   string      `xml:"comment"`
}

type arxivAuthor struct {
	Name string `xml:"name"`
}

type arxivLink struct {
	Href  string `xml:"href,attr"`
	Title string `xml:"title,attr"`
	Type  string `xml:"type,attr"`
}

type arxivCat struct {
	Term string `xml:"term,attr"`
}

func (e *arxivEngine) parse(data []byte) (*antirobot.SearchResponse, error) {
	var feed arxivFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, fmt.Errorf("arxiv parse: %w", err)
	}

	results := make([]antirobot.Result, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		if entry.ID == "" {
			continue
		}

		authors := make([]string, 0, len(entry.Authors))
		for _, a := range entry.Authors {
			if a.Name != "" {
				authors = append(authors, a.Name)
			}
		}

		pdfURL := ""
		for _, lnk := range entry.Links {
			if lnk.Title == "pdf" {
				pdfURL = lnk.Href
				break
			}
		}

		title := antirobot.CollapseSpace(strings.TrimSpace(entry.Title))
		summary := antirobot.CollapseSpace(strings.TrimSpace(entry.Summary))

		results = append(results, antirobot.Result{
			Type:        antirobot.ResultPaper,
			Title:       title,
			URL:         entry.ID,
			Content:     summary,
			PDFURL:      pdfURL,
			Authors:     strings.Join(authors, ", "),
			PublishedAt: entry.Published[:min(len(entry.Published), 10)],
			DOI:         entry.DOI,
			Engine:      "arxiv",
		})
	}

	return &antirobot.SearchResponse{Engine: "arxiv", Results: results}, nil
}
