package academic

import (
	"strings"
	"testing"

	"websearch/pkg/antirobot"
)

func TestPubMedSearch(t *testing.T) {
	engine := NewPubMed(antirobot.PubMedOpts{Enabled: true}, nil)
	resp, err := engine.Search("mRNA vaccine efficacy", 1, antirobot.TimeRangeNone)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if resp.Engine != "pubmed" {
		t.Errorf("engine = %q, want pubmed", resp.Engine)
	}
	if len(resp.Results) == 0 {
		t.Fatal("expected results, got 0")
	}

	r := resp.Results[0]
	t.Logf("Title: %s", r.Title)
	t.Logf("URL: %s", r.URL)
	t.Logf("DOI: %s", r.DOI)
	t.Logf("Journal: %s", r.Journal)
	t.Logf("Authors: %s", r.Authors)
	t.Logf("Content: %s", r.Content[:min(len(r.Content), 100)])

	if r.Type != antirobot.ResultPaper {
		t.Errorf("type = %q, want paper", r.Type)
	}
	if r.Title == "" {
		t.Error("title is empty")
	}
	if !strings.HasPrefix(r.URL, "https://www.ncbi.nlm.nih.gov/pubmed/") {
		t.Errorf("url = %q, should start with pubmed URL", r.URL)
	}
	if r.DOI == "" {
		t.Log("WARN: DOI is empty (some articles may not have DOI)")
	}
	if r.Journal == "" {
		t.Error("journal is empty")
	}
	if r.Authors == "" {
		t.Error("authors is empty")
	}

	// 验证 markdown 格式
	md := r.Markdown()
	t.Logf("\n--- Markdown Output ---\n%s", md)
	if !strings.Contains(md, r.Title) {
		t.Error("markdown missing title")
	}
	if r.DOI != "" && !strings.Contains(md, "DOI:") {
		t.Error("markdown missing DOI")
	}
	if !strings.Contains(md, r.Journal) {
		t.Error("markdown missing journal")
	}
}

func TestPubMedSearch_NoResults(t *testing.T) {
	engine := NewPubMed(antirobot.PubMedOpts{Enabled: true}, nil)
	resp, err := engine.Search("zzzznonexistentquery1234567890abcdef", 1, antirobot.TimeRangeNone)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Errorf("expected 0 results for nonsense query, got %d", len(resp.Results))
	}
}
