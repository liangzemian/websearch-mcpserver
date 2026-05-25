package academic

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"websearch/pkg/antirobot"
)

// ──────────────────────────────────────────────────────────────────────────────
// Semantic Scholar 学术搜索（海外优先）
// ──────────────────────────────────────────────────────────────────────────────

type semanticScholarEngine struct {
	client *http.Client
}

// NewSemanticScholar 创建 Semantic Scholar 引擎。client 为 nil 时使用默认客户端。
func NewSemanticScholar(_ antirobot.SemanticScholarOpts, client *http.Client) antirobot.Engine {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &semanticScholarEngine{client: client}
}

func (e *semanticScholarEngine) Name() string                    { return "semantic_scholar" }
func (e *semanticScholarEngine) Region() antirobot.NetworkRegion { return antirobot.RegionInternational }

func (e *semanticScholarEngine) Search(query string, page int, timeRange antirobot.TimeRange) (*antirobot.SearchResponse, error) {
	offset := (page - 1) * 10
	if offset < 0 {
		offset = 0
	}

	params := url.Values{
		"query":  {query},
		"offset": {fmt.Sprintf("%d", offset)},
		"limit":  {"10"},
		"fields": {"title,url,abstract,authors,year,externalIds,venue,citationCount,openAccessPdf,relevanceScore"},
	}
	if since := antirobot.TimeRangeSince(timeRange); since != "" {
		year := since[:4]
		params.Set("year", year+"-")
	}

	u := "https://api.semanticscholar.org/graph/v1/paper/search?" + params.Encode()

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "websearch/1.0")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("semantic scholar request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("semantic scholar HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return e.parse(body)
}

// ── JSON 解析 ──

type ssResponse struct {
	Total int       `json:"total"`
	Data  []ssPaper `json:"data"`
}

type ssPaper struct {
	PaperID        string     `json:"paperId"`
	Title          string     `json:"title"`
	URL            string     `json:"url"`
	Abstract       string     `json:"abstract"`
	Year           int        `json:"year"`
	Venue          string     `json:"venue"`
	CitationCount  int        `json:"citationCount"`
	RelevanceScore float64    `json:"relevanceScore"`
	Authors        []ssAuthor `json:"authors"`
	ExternalIDs    ssExtIDs   `json:"externalIds"`
	OpenAccess     *ssOA      `json:"openAccessPdf"`
}

type ssAuthor struct {
	Name string `json:"name"`
}

type ssExtIDs struct {
	DOI   string `json:"DOI"`
	ArXiv string `json:"ArXiv"`
}

type ssOA struct {
	URL string `json:"url"`
}

func (e *semanticScholarEngine) parse(data []byte) (*antirobot.SearchResponse, error) {
	var resp ssResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("semantic scholar parse: %w", err)
	}

	results := make([]antirobot.Result, 0, len(resp.Data))
	for _, p := range resp.Data {
		if p.Title == "" {
			continue
		}

		resultURL := p.URL
		if resultURL == "" && p.ExternalIDs.DOI != "" {
			resultURL = "https://doi.org/" + p.ExternalIDs.DOI
		}

		pdfURL := ""
		if p.OpenAccess != nil && p.OpenAccess.URL != "" {
			pdfURL = p.OpenAccess.URL
		}

		authors := make([]string, 0, len(p.Authors))
		for _, a := range p.Authors {
			if a.Name != "" {
				authors = append(authors, a.Name)
			}
		}

		pubDate := ""
		if p.Year > 0 {
			pubDate = fmt.Sprintf("%d", p.Year)
		}

		title := antirobot.CollapseSpace(strings.TrimSpace(p.Title))
		abstract := antirobot.CollapseSpace(strings.TrimSpace(p.Abstract))

		results = append(results, antirobot.Result{
			Type:        antirobot.ResultPaper,
			Title:       title,
			URL:         resultURL,
			Content:     abstract,
			PDFURL:      pdfURL,
			Authors:     strings.Join(authors, ", "),
			PublishedAt: pubDate,
			DOI:         p.ExternalIDs.DOI,
			Journal:     p.Venue,
			CitedBy:     p.CitationCount,
			Score:       p.RelevanceScore,
			Engine:      "semantic_scholar",
		})
	}

	return &antirobot.SearchResponse{Engine: "semantic_scholar", Results: results}, nil
}
