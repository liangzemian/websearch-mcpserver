package antirobot

// ── 学术引擎 Options ──

// ArxivOpts arXiv 预印本配置。
type ArxivOpts struct {
	Enabled bool
}

// CrossrefOpts Crossref 学术元数据配置。
type CrossrefOpts struct {
	Enabled bool
}

// OpenAlexOpts OpenAlex 开放学术图谱配置。
type OpenAlexOpts struct {
	Enabled bool
	MailTo  string // polite pool 邮箱（可选）
}

// SemanticScholarOpts Semantic Scholar 配置。
type SemanticScholarOpts struct {
	Enabled bool
}

// PubMedOpts PubMed 生物医学文献配置。
type PubMedOpts struct {
	Enabled bool
}

// GoogleScholarOpts Google Scholar 学术搜索配置。
type GoogleScholarOpts struct {
	Enabled bool
	Domain  string // 可选自定义域名（默认 scholar.google.com）
}
