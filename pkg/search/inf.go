package search

import "websearch/pkg/antirobot"

var DefaultSearchInf SearchInf

type SearchResult struct {
	Title       string `json:"title"`
	Url         string `json:"url"`
	Content     string `json:"content"`
	PublishDate string `json:"publishedDate"`
	Type        string `json:"type,omitempty"`       // "paper" 或 "web"，学术搜索时为 "paper"
	Authors     string `json:"authors,omitempty"`     // 论文作者
	DOI         string `json:"doi,omitempty"`         // 论文 DOI
	Journal     string `json:"journal,omitempty"`     // 期刊/会议名
	CitedBy     int    `json:"cited_by,omitempty"`    // 被引次数
	PDFURL      string `json:"pdf_url,omitempty"`     // PDF 链接
}

type SearchInf interface {
	Search(query string) (string, error)
	SearchRaw(query string) ([]SearchResult, error)
	MergeContent(query string, results []SearchResult) (string, error)
}

// AcademicSearchOptions 学术搜索可选参数。
type AcademicSearchOptions struct {
	Page      int      // 页码（默认 1）
	TimeRange string   // 时间范围: "year", "all"（默认 "all"）
	Engines   []string // 指定引擎子集（为空则使用全部已启用引擎）
}

// AcademicSearcher 支持学术搜索的引擎可实现此接口。
type AcademicSearcher interface {
	SearchAcademicRaw(query string, opts ...AcademicSearchOptions) ([]SearchResult, error)
	AcademicEngines() []string // 返回可用的学术引擎列表
}

// ParseTimeRange 将字符串转换为 antirobot.TimeRange。
func ParseTimeRange(s string) antirobot.TimeRange {
	switch s {
	case "day":
		return antirobot.TimeRangeDay
	case "week":
		return antirobot.TimeRangeWeek
	case "month":
		return antirobot.TimeRangeMonth
	case "year":
		return antirobot.TimeRangeYear
	default:
		return antirobot.TimeRangeNone
	}
}
