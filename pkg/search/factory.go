package search

import (
	"fmt"
	"websearch/pkg/antirobot"
	"websearch/pkg/baidu"
	"websearch/pkg/bing"
	"websearch/pkg/config"
	"websearch/pkg/google"
	"websearch/pkg/log"
)

// SearchGroup 搜索引擎组，包含主引擎、兜底引擎和学术引擎。
type SearchGroup struct {
	Primary  SearchInf          // 主搜索引擎
	Fallback *BingSearchAdapter // Bing 兜底引擎（可为 nil）
	Academic AcademicSearcher   // 学术搜索引擎（可为 nil）
	conf     config.Config      // 保存配置，用于代理变更时重建引擎
}

// NewFromConfig 根据配置初始化搜索引擎组。
func NewFromConfig(conf config.Config) (*SearchGroup, error) {
	g := &SearchGroup{conf: conf}

	// ShowMeta 配置
	ShowMeta = conf.SmartSearch.ShowMeta

	// ── 初始化 Bing 引擎（兜底） ──
	initBingEngine(conf, g)

	// ── 初始化百度网页搜索引擎（无需 API Key，SK 失败时回退） ──
	baiduWebAdapter := initBaiduWebEngine(conf)

	// ── 初始化 Google 引擎（需代理，由 resolver 动态解析） ──
	googleAdapter := initGoogleEngine(conf)

	// ── 按模式选择主引擎 ──
	switch conf.GetMode() {
	case config.ModeEngine:
		var engines []SearchInf
		if baiduWebAdapter != nil {
			engines = append(engines, baiduWebAdapter)
		}
		if g.Fallback != nil {
			engines = append(engines, g.Fallback)
		}
		if googleAdapter != nil {
			engines = append(engines, googleAdapter)
		}
		if len(engines) == 0 {
			return nil, fmt.Errorf("engine 模式需要至少一个引擎，请检查 bing 配置")
		}
		if len(engines) == 1 {
			g.Primary = engines[0]
		} else {
			hs := NewHybridSearch(engines...)
			applySmartSearchFilters(hs, conf)
			g.Primary = hs
		}
		log.Infof("搜索模式: engine（无需 API Key，%d 个引擎并发）", len(engines))

	case config.ModeTavily:
		if conf.Tavily.APIKey == "" {
			log.Error("mode=tavily 但未配置 tavily.api_key，回退到 engine 模式")
			if g.Fallback != nil {
				g.Primary = g.Fallback
			} else {
				return nil, fmt.Errorf("无可用搜索引擎")
			}
		} else {
			g.Primary = NewTavilySearch(conf.Tavily.APIKey, conf.BlackListHost)
		}

	case config.ModeHybrid:
		var engines []SearchInf
		if conf.Baidu.APIKey != "" {
			engines = append(engines, NewBaiduSeach(conf.Baidu.APIKey, conf.BlackListHost))
		}
		if baiduWebAdapter != nil {
			engines = append(engines, baiduWebAdapter)
		}
		if conf.Tavily.APIKey != "" {
			engines = append(engines, NewTavilySearch(conf.Tavily.APIKey, conf.BlackListHost))
		}
		// Bing 作为原生引擎参与混合搜索
		if g.Fallback != nil {
			engines = append(engines, g.Fallback)
		}
		if googleAdapter != nil {
			engines = append(engines, googleAdapter)
		}
		if len(engines) == 0 {
			return nil, fmt.Errorf("无可用搜索引擎，请检查配置")
		}
		hs := NewHybridSearch(engines...)
		applySmartSearchFilters(hs, conf)
		g.Primary = hs

	default: // baidu
		if conf.Baidu.APIKey != "" {
			primary := NewBaiduSeach(conf.Baidu.APIKey, conf.BlackListHost)
			if baiduWebAdapter != nil {
				g.Primary = NewBaiduWithFallback(primary, baiduWebAdapter)
				log.Info("搜索模式: baidu（千帆 SK + 网页搜索回退）")
			} else {
				g.Primary = primary
			}
		} else if baiduWebAdapter != nil {
			g.Primary = baiduWebAdapter
			log.Info("搜索模式: baidu（网页搜索，无需 API Key）")
		} else if g.Fallback != nil {
			log.Error("mode=baidu 但未配置 baidu.api_key 且无可用引擎，回退到 Bing")
			g.Primary = g.Fallback
		} else {
			return nil, fmt.Errorf("无可用搜索引擎")
		}
	}

	log.Infof("搜索模式: %s", conf.GetMode())

	// ── 学术搜索引擎（独立于主引擎） ──
	initAcademicEngine(conf, g)

	return g, nil
}

// initBaiduWebEngine 初始化百度网页搜索引擎。
func initBaiduWebEngine(conf config.Config) *EngineSearchAdapter {
	blocked := bing.MergeBlocked(conf.BlackListHost, nil)
	eng := baidu.NewBaiduWeb(baidu.BaiduOpts{
		Enabled: true,
		Blocked: blocked,
		PerSec:  conf.GetRateLimitPerSec(),
		PerMin:  conf.GetRateLimitPerMin(),
	})
	adapter := NewEngineSearchAdapter("baidu", eng)
	log.Info("百度网页搜索引擎已启用（tn=json，无需 API Key）")
	return adapter
}

// initGoogleEngine 初始化 Google 引擎。
// 始终初始化，代理端点由 resolver 在每次请求时动态解析。
func initGoogleEngine(conf config.Config) *EngineSearchAdapter {
	blocked := bing.MergeBlocked(conf.BlackListHost, nil)
	eng := google.NewGoogle(google.GoogleOpts{
		Enabled:      true,
		Blocked:      blocked,
		ProxyResolve: conf.Proxy.ProxyResolver(),
		PerSec:       conf.GetRateLimitPerSec(),
		PerMin:       conf.GetRateLimitPerMin(),
	})
	adapter := NewEngineSearchAdapter("google", eng)
	if conf.Proxy.ProxyResolver() != nil {
		log.Info("Google 引擎已启用（代理: 自动检测）")
	} else {
		log.Info("Google 引擎已启用（直连）")
	}
	return adapter
}

// initBingEngine 根据配置初始化 Bing 引擎适配器。
func initBingEngine(conf config.Config, g *SearchGroup) {
	if !conf.Bing.Enabled {
		log.Info("Bing 引擎已禁用（bing.enabled=false）")
		return
	}

	bingOpts := bing.BingOpts{
		Enabled: true,
		Blocked: bing.MergeBlocked(conf.BlackListHost, conf.Bing.Blocked),
		PerSec:  conf.GetRateLimitPerSec(),
		PerMin:  conf.GetRateLimitPerMin(),
	}

	g.Fallback = NewBingSearchAdapter(bingOpts)
	log.Infof("Bing 引擎已启用，引擎: %v", g.Fallback.Engines())
}

// initAcademicEngine 根据配置初始化学术搜索引擎。
// 始终初始化所有未显式禁用的引擎，代理端点由 resolver 在每次请求时动态解析。
func initAcademicEngine(conf config.Config, g *SearchGroup) {
	acad := conf.Academic
	if !acad.Enabled {
		log.Info("学术引擎未启用（academic.enabled=false）")
		return
	}

	network := antirobot.RegionChina
	if conf.IsInternational() {
		network = antirobot.RegionInternational
	}

	acadConf := AcademicConfig{
		Network:         network,
		Arxiv:           antirobot.ArxivOpts{Enabled: !acad.DisableArxiv},
		Crossref:        antirobot.CrossrefOpts{Enabled: !acad.DisableCrossref},
		OpenAlex:        antirobot.OpenAlexOpts{Enabled: !acad.DisableOpenAlex},
		SemanticScholar: antirobot.SemanticScholarOpts{Enabled: !acad.DisableSemanticScholar},
		PubMed:          antirobot.PubMedOpts{Enabled: !acad.DisablePubMed},
		GoogleScholar:   antirobot.GoogleScholarOpts{Enabled: !acad.DisableGoogleScholar},
		ProxyResolve:    conf.Proxy.ProxyResolver(),
	}

	adapter := NewAcademicAdapter(acadConf)
	if adapter == nil {
		log.Info("无可用学术引擎")
		return
	}

	g.Academic = adapter
	engines := adapter.Engines()
	log.Infof("学术引擎已启用: %v", engines)
}

// applySmartSearchFilters 将 SmartSearchConfig 转换为 HybridSearchImpl 的过滤规则。
func applySmartSearchFilters(hs *HybridSearchImpl, conf config.Config) {
	sc := conf.SmartSearch
	if sc.MaxSize > 0 {
		hs.SetMaxSize(sc.MaxSize)
	}
	if len(sc.Engines) == 0 {
		return
	}
	engineMap := make(map[string]engineFilter, len(sc.Engines))
	for name, ec := range sc.Engines {
		engineMap[name] = engineFilter{
			minScore: ec.MinScore,
			maxSize:  ec.MaxSize,
		}
	}
	hs.SetFilters(engineMap)
}
