package search

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"websearch/pkg/academic"
	"websearch/pkg/antirobot"
	md "websearch/pkg/xml"
)

// ──────────────────────────────────────────────────────────────────────────────
// AcademicAdapter 学术搜索引擎适配器
// ──────────────────────────────────────────────────────────────────────────────

// AcademicAdapter 将学术引擎适配为 AcademicSearcher 接口。
// 负责 arXiv、Crossref、OpenAlex、Semantic Scholar、PubMed、Google Scholar。
type AcademicAdapter struct {
	searcher *antirobot.Searcher
	engines  []antirobot.Engine // 保存全部引擎引用，用于按名过滤
}

// AcademicConfig 学术引擎配置。
type AcademicConfig struct {
	Network         antirobot.NetworkRegion
	Arxiv           antirobot.ArxivOpts
	Crossref        antirobot.CrossrefOpts
	OpenAlex        antirobot.OpenAlexOpts
	SemanticScholar antirobot.SemanticScholarOpts
	PubMed          antirobot.PubMedOpts
	GoogleScholar   antirobot.GoogleScholarOpts
	Proxy           string // 代理端点
}

// NewAcademicAdapter 创建学术搜索适配器。
func NewAcademicAdapter(conf AcademicConfig) *AcademicAdapter {
	engines := academic.BuildAcademic(struct {
		Network         antirobot.NetworkRegion
		Arxiv           antirobot.ArxivOpts
		Crossref        antirobot.CrossrefOpts
		OpenAlex        antirobot.OpenAlexOpts
		SemanticScholar antirobot.SemanticScholarOpts
		PubMed          antirobot.PubMedOpts
		GoogleScholar   antirobot.GoogleScholarOpts
		Proxy           string
	}{
		Network:         conf.Network,
		Arxiv:           conf.Arxiv,
		Crossref:        conf.Crossref,
		OpenAlex:        conf.OpenAlex,
		SemanticScholar: conf.SemanticScholar,
		PubMed:          conf.PubMed,
		GoogleScholar:   conf.GoogleScholar,
		Proxy:           conf.Proxy,
	})

	if len(engines) == 0 {
		return nil
	}

	searcher := antirobot.NewSearcher(antirobot.StrategyParallel, engines)
	return &AcademicAdapter{searcher: searcher, engines: engines}
}

// SearchAcademicRaw 实现 AcademicSearcher 接口，返回学术论文搜索结果。
func (a *AcademicAdapter) SearchAcademicRaw(query string, opts ...AcademicSearchOptions) ([]SearchResult, error) {
	var opt AcademicSearchOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	if opt.Page <= 0 {
		opt.Page = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 设置时间范围
	tr := ParseTimeRange(opt.TimeRange)
	a.searcher.TimeRange = tr

	// 按引擎名过滤
	searcher := a.searcher
	if len(opt.Engines) > 0 {
		filtered := a.filterEngines(opt.Engines)
		if len(filtered) == 0 {
			return nil, fmt.Errorf("指定的引擎均不可用: %v", opt.Engines)
		}
		searcher = antirobot.NewSearcher(antirobot.StrategyParallel, filtered)
		searcher.TimeRange = tr
	}

	responses := searcher.Search(ctx, query, opt.Page)

	var all []antirobot.Result
	for _, resp := range responses {
		if resp.Error != "" {
			continue
		}
		all = append(all, resp.Results...)
	}
	all = antirobot.DeduplicateResults(all)
	all = antirobot.NormalizeAndSortResults(all)

	if len(all) == 0 {
		return nil, fmt.Errorf("学术引擎搜索无结果")
	}

	results := make([]SearchResult, 0, len(all))
	for _, r := range all {
		results = append(results, SearchResult{
			Title:       r.Title,
			Url:         strings.TrimSpace(r.URL),
			Content:     r.Content,
			PublishDate: r.PublishedAt,
			Type:        string(r.Type),
			Authors:     r.Authors,
			DOI:         r.DOI,
			Journal:     r.Journal,
			CitedBy:     r.CitedBy,
			PDFURL:      r.PDFURL,
		})
	}
	return results, nil
}

// MergeContent 格式化学术搜索结果为 Markdown。
func (a *AcademicAdapter) MergeContent(query string, results []SearchResult) (string, error) {
	if len(results) == 0 {
		return "", fmt.Errorf("没有搜索结果")
	}
	var buf strings.Builder
	buf.Grow(1024 * len(results))
	buf.WriteString(md.MDSearchHeader(query, len(results)))
	for i, val := range results {
		if val.Type == "paper" {
			citedByStr := ""
			if val.CitedBy > 0 {
				citedByStr = strconv.Itoa(val.CitedBy)
			}
			buf.WriteString(md.FormatPaperMD(i+1, val.Title, val.Url,
				val.Authors, val.DOI, val.Journal, val.PublishDate,
				val.PDFURL, citedByStr, val.Content))
		} else {
			buf.WriteString(md.FormatMD(i+1, val.Title, val.Url, val.Content))
		}
	}
	return buf.String(), nil
}

// Engines 返回已注册的学术引擎名称列表。
func (a *AcademicAdapter) Engines() []string {
	return a.searcher.Engines()
}

// AcademicEngines 实现 AcademicSearcher 接口。
func (a *AcademicAdapter) AcademicEngines() []string {
	return a.searcher.Engines()
}

// filterEngines 按名称过滤引擎子集。
func (a *AcademicAdapter) filterEngines(names []string) []antirobot.Engine {
	nameSet := make(map[string]struct{}, len(names))
	for _, n := range names {
		nameSet[strings.ToLower(strings.TrimSpace(n))] = struct{}{}
	}
	var filtered []antirobot.Engine
	for _, eng := range a.engines {
		if _, ok := nameSet[eng.Name()]; ok {
			filtered = append(filtered, eng)
		}
	}
	return filtered
}
