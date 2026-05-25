package academic

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"websearch/pkg/antirobot"

	"github.com/PuerkitoBio/goquery"
)

// ──────────────────────────────────────────────────────────────────────────────
// Google Scholar 学术搜索（海外优先）
// ──────────────────────────────────────────────────────────────────────────────

const defaultScholarDomain = "scholar.google.com"

type googleScholarEngine struct {
	client *http.Client
	domain string
}

// NewGoogleScholar 创建 Google Scholar 引擎。client 为 nil 时使用默认客户端。
func NewGoogleScholar(opts antirobot.GoogleScholarOpts, client *http.Client) antirobot.Engine {
	domain := opts.Domain
	if domain == "" {
		domain = defaultScholarDomain
	}
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &googleScholarEngine{
		client: client,
		domain: domain,
	}
}

func (e *googleScholarEngine) Name() string                    { return "google_scholar" }
func (e *googleScholarEngine) Region() antirobot.NetworkRegion { return antirobot.RegionInternational }

func (e *googleScholarEngine) Search(query string, page int, timeRange antirobot.TimeRange) (*antirobot.SearchResponse, error) {
	start := (page - 1) * 10
	if start < 0 {
		start = 0
	}

	params := url.Values{
		"q":      {query},
		"start":  {strconv.Itoa(start)},
		"as_sdt": {"2007"},
		"as_vis": {"0"},
		"hl":     {"en"},
	}
	if year := scholarTimeRangeYear(timeRange); year > 0 {
		params.Set("as_ylo", strconv.Itoa(year))
	}

	scholarURL := fmt.Sprintf("https://%s/scholar?%s", e.domain, params.Encode())

	req, err := http.NewRequest("GET", scholarURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google scholar request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		loc := resp.Header.Get("Location")
		if strings.Contains(loc, "/sorry") {
			return nil, fmt.Errorf("google scholar: access denied (CAPTCHA redirect)")
		}
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("google scholar HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseScholarHTML(body)
}

// ── HTML 解析 ──

func parseScholarHTML(data []byte) (*antirobot.SearchResponse, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("google scholar parse: %w", err)
	}

	if doc.Find("form#gs_captcha_f").Length() > 0 {
		return nil, fmt.Errorf("google scholar: CAPTCHA detected")
	}

	var results []antirobot.Result
	doc.Find("div[data-rp]").Each(func(_ int, sel *goquery.Selection) {
		title := strings.TrimSpace(sel.Find("h3").First().Find("a").First().Text())
		if title == "" {
			return
		}

		href, _ := sel.Find("h3").First().Find("a").First().Attr("href")

		content := antirobot.CollapseSpace(strings.TrimSpace(sel.Find("div.gs_rs").Text()))

		authorsStr := sel.Find("div.gs_a").Text()
		authors, journal, _, pubDate := parseScholarMeta(authorsStr)

		citedByText := sel.Find("div.gs_fl a").FilterFunction(func(_ int, s *goquery.Selection) bool {
			h, _ := s.Attr("href")
			return strings.HasPrefix(h, "/scholar?cites=")
		}).Text()
		citedBy := parseCitedByText(citedByText)

		pdfURL := ""
		docLink := sel.Find("div.gs_or_ggsm a")
		if docLink.Length() > 0 {
			docHref, _ := docLink.First().Attr("href")
			docType := strings.TrimSpace(sel.Find("span.gs_ctg2").Text())
			if docType == "[PDF]" {
				pdfURL = docHref
			}
		}

		title = antirobot.CollapseSpace(title)
		results = append(results, antirobot.Result{
			Type:        antirobot.ResultPaper,
			Title:       title,
			URL:         href,
			Content:     content,
			Authors:     authors,
			Journal:     journal,
			PublishedAt: pubDate,
			CitedBy:     citedBy,
			PDFURL:      pdfURL,
			Engine:      "google_scholar",
		})
	})

	return &antirobot.SearchResponse{Engine: "google_scholar", Results: results}, nil
}

func parseScholarMeta(text string) (authors string, journal string, publisher string, pubDate string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", "", ""
	}

	parts := strings.SplitN(text, " - ", 3)

	authorList := strings.Split(parts[0], ", ")
	for i := range authorList {
		authorList[i] = strings.TrimSpace(authorList[i])
	}
	authors = strings.Join(authorList, ", ")

	if len(parts) < 2 {
		return authors, "", "", ""
	}

	publisher = strings.TrimSpace(parts[len(parts)-1])

	if len(parts) < 3 {
		return authors, "", publisher, ""
	}

	middle := strings.TrimSpace(parts[1])
	commaIdx := strings.LastIndex(middle, ",")
	if commaIdx > 0 {
		journal = strings.TrimSpace(middle[:commaIdx])
		yearStr := strings.TrimSpace(middle[commaIdx+1:])
		if y, err := strconv.Atoi(yearStr); err == nil && y > 1900 && y < 2100 {
			pubDate = strconv.Itoa(y)
		}
	} else {
		if y, err := strconv.Atoi(middle); err == nil && y > 1900 && y < 2100 {
			pubDate = strconv.Itoa(y)
		}
	}

	return authors, journal, publisher, pubDate
}

func parseCitedByText(text string) int {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "Cited by")
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	n, _ := strconv.Atoi(text)
	return n
}

func scholarTimeRangeYear(tr antirobot.TimeRange) int {
	if tr == antirobot.TimeRangeNone {
		return 0
	}
	return time.Now().Year() - 1
}
