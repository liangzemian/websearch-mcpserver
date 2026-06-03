package proxy

import (
	"sync"
	"time"
)

// Detector 后台轮询检测系统代理变更，并在变更时通知回调。
type Detector struct {
	mu        sync.RWMutex
	current   string
	fallback  string
	callbacks []func(endpoint string)
	logf      func(format string, args ...any)
	stopCh    chan struct{}
	once      sync.Once
}

// NewDetector 创建系统代理变更检测器。
// fallback 为系统检测失败时的回退端点（可为空）。
func NewDetector(fallback string) *Detector {
	return &Detector{
		fallback: fallback,
		logf:     func(string, ...any) {},
		stopCh:   make(chan struct{}),
	}
}

// SetLogger 设置日志函数。
func (d *Detector) SetLogger(logf func(format string, args ...any)) {
	d.logf = logf
}

// Endpoint 返回当前生效的代理端点。
func (d *Detector) Endpoint() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.current
}

// OnChange 注册代理变更回调。
func (d *Detector) OnChange(fn func(endpoint string)) {
	d.mu.Lock()
	d.callbacks = append(d.callbacks, fn)
	d.mu.Unlock()
}

// Start 启动后台轮询检测。interval 为检测间隔，建议 30s。
func (d *Detector) Start(interval time.Duration) {
	d.detect() // 初始检测一次
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-d.stopCh:
				return
			case <-ticker.C:
				d.detect()
			}
		}
	}()
}

// Stop 停止后台轮询。
func (d *Detector) Stop() {
	d.once.Do(func() { close(d.stopCh) })
}

func (d *Detector) detect() {
	newEP := DetectSystemProxy()
	if newEP == "" {
		newEP = d.fallback
	}

	d.mu.Lock()
	old := d.current
	d.current = newEP
	d.mu.Unlock()

	if old == newEP {
		return
	}

	d.logf("代理变更: %s → %s", showEP(old), showEP(newEP))

	d.mu.RLock()
	cbs := make([]func(string), len(d.callbacks))
	copy(cbs, d.callbacks)
	d.mu.RUnlock()
	for _, cb := range cbs {
		cb(newEP)
	}
}

func showEP(ep string) string {
	if ep == "" {
		return "无"
	}
	return ep
}
