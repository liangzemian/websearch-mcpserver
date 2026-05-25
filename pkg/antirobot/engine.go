package antirobot

// Engine 搜索引擎接口。
type Engine interface {
	Name() string
	Region() NetworkRegion
	Search(query string, page int, timeRange TimeRange) (*SearchResponse, error)
}
