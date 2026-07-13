package search

import (
	"testing"
)

// ── 集成测试（从 config.test.yaml 加载 API Key） ──

func TestBaiduSearchImpl_SearchRaw_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试: -short 模式")
	}
	apiKey := loadBaiduAPIKey(t)

	baidu := NewBaiduSeach(apiKey, []string{"csdn.net"})
	results, err := baidu.SearchRaw("Go programming language")
	if err != nil {
		t.Fatalf("SearchRaw failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected non-empty results")
	}
	for i, r := range results {
		t.Logf("[%d] %s - %s", i+1, r.Title, r.Url)
		if r.Title == "" {
			t.Error("result title should not be empty")
		}
		if r.Url == "" {
			t.Error("result url should not be empty")
		}
		if r.Engine != "baidu_api" {
			t.Errorf("expected engine 'baidu_api', got %s", r.Engine)
		}
	}
}

func TestBaiduSearchImpl_Search_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试: -short 模式")
	}
	apiKey := loadBaiduAPIKey(t)

	baidu := NewBaiduSeach(apiKey, nil)
	output, err := baidu.Search("latest AI news")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if output == "" {
		t.Fatal("expected non-empty output")
	}
	t.Logf("Search output:\n%s", output)
}

func TestBaiduSearchImpl_SearchRawWithTimeRange_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试: -short 模式")
	}
	apiKey := loadBaiduAPIKey(t)

	baidu := NewBaiduSeach(apiKey, nil)

	// 测试时间范围: day
	results, err := baidu.SearchRawWithTimeRange("AI news", 1)
	if err != nil {
		t.Fatalf("SearchRawWithTimeRange(day) failed: %v", err)
	}
	t.Logf("day 范围结果数: %d", len(results))

	// 测试时间范围: week
	results, err = baidu.SearchRawWithTimeRange("AI news", 7)
	if err != nil {
		t.Fatalf("SearchRawWithTimeRange(week) failed: %v", err)
	}
	t.Logf("week 范围结果数: %d", len(results))

	// 测试时间范围: month
	results, err = baidu.SearchRawWithTimeRange("AI news", 30)
	if err != nil {
		t.Fatalf("SearchRawWithTimeRange(month) failed: %v", err)
	}
	t.Logf("month 范围结果数: %d", len(results))

	// 测试时间范围: semiyear
	results, err = baidu.SearchRawWithTimeRange("AI news", 180)
	if err != nil {
		t.Fatalf("SearchRawWithTimeRange(semiyear) failed: %v", err)
	}
	t.Logf("semiyear 范围结果数: %d", len(results))

	// 测试默认时间范围（使用引擎默认值）
	results, err = baidu.SearchRawWithTimeRange("AI news", 0)
	if err != nil {
		t.Fatalf("SearchRawWithTimeRange(default) failed: %v", err)
	}
	t.Logf("默认范围结果数: %d", len(results))
}

func TestLookbackDaysToRecency(t *testing.T) {
	tests := []struct {
		days int
		want string
	}{
		{0, "semiyear"},  // 默认
		{1, "day"},
		{3, "week"},
		{7, "week"},
		{15, "month"},
		{30, "month"},
		{90, "semiyear"},
		{180, "semiyear"},
		{365, "year"},
	}
	for _, tt := range tests {
		got := lookbackDaysToRecency(tt.days)
		if got != tt.want {
			t.Errorf("lookbackDaysToRecency(%d) = %s, want %s", tt.days, got, tt.want)
		}
	}
}
