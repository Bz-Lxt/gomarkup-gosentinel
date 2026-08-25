package telemetry

import (
	"sync"
	"time"

	"gosentinel/internal/protocol"
	"gosentinel/internal/timeutil"
)

type sample struct {
	At         time.Time
	Pass       uint64
	Block      uint64
	Error      uint64
	Fallback   uint64
	BlockRatio float64
	ErrorRatio float64
	QPS        float64
	State      string
}

type seriesKey struct {
	Service  string
	Instance string
	Resource string
}

type Aggregator struct {
	mu      sync.Mutex
	series  map[seriesKey][]sample
	dropped uint64
}

func New() *Aggregator {
	return &Aggregator{series: make(map[seriesKey][]sample)}
}

func (a *Aggregator) Ingest(service, instance string, tel protocol.Telemetry) {
	now := timeutil.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, p := range tel.Resources {
		k := seriesKey{Service: service, Instance: instance, Resource: p.Resource}
		arr := a.series[k]
		arr = append(arr, sample{
			At: now, Pass: p.Pass, Block: p.Block, Error: p.Error, Fallback: p.Fallback,
			BlockRatio: p.BlockRatio, ErrorRatio: p.ErrorRatio, QPS: p.QPS, State: p.State,
		})
		if len(arr) > 180 {
			arr = arr[len(arr)-180:]
		}
		a.series[k] = arr
	}
}

func (a *Aggregator) Dropped() uint64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.dropped
}

type Point struct {
	At         string  `json:"at"`
	Pass       uint64  `json:"pass"`
	Block      uint64  `json:"block"`
	Error      uint64  `json:"error"`
	Fallback   uint64  `json:"fallback"`
	BlockRatio float64 `json:"block_ratio"`
	ErrorRatio float64 `json:"error_ratio"`
	QPS        float64 `json:"qps"`
	State      string  `json:"state"`
}

type ResourceView struct {
	Service    string  `json:"service"`
	Instance   string  `json:"instance"`
	Resource   string  `json:"resource"`
	Pass       uint64  `json:"pass"`
	Block      uint64  `json:"block"`
	Error      uint64  `json:"error"`
	Fallback   uint64  `json:"fallback"`
	QPS        float64 `json:"qps"`
	BlockRatio float64 `json:"block_ratio"`
	ErrorRatio float64 `json:"error_ratio"`
	State      string  `json:"state"`
	Series     []Point `json:"series"`
}

func (a *Aggregator) Query(service, resource, instance string, window time.Duration) []ResourceView {
	if window <= 0 {
		window = 60 * time.Second
	}
	cut := timeutil.Now().Add(-window)
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]ResourceView, 0, len(a.series))
	for k, arr := range a.series {
		if service != "" && service != k.Service {
			continue
		}
		if resource != "" && resource != k.Resource {
			continue
		}
		if instance != "" && instance != k.Instance {
			continue
		}
		v := ResourceView{Service: k.Service, Instance: k.Instance, Resource: k.Resource}
		for _, s := range arr {
			if s.At.Before(cut) {
				continue
			}
			v.Series = append(v.Series, Point{
				At: timeutil.Format(s.At), Pass: s.Pass, Block: s.Block, Error: s.Error, Fallback: s.Fallback,
				BlockRatio: s.BlockRatio, ErrorRatio: s.ErrorRatio, QPS: s.QPS, State: s.State,
			})
			v.Pass += s.Pass
			v.Block += s.Block
			v.Error += s.Error
			v.Fallback += s.Fallback
			v.QPS = s.QPS
			v.BlockRatio = s.BlockRatio
			v.ErrorRatio = s.ErrorRatio
			v.State = s.State
		}
		if v.Series == nil {
			v.Series = []Point{}
		}
		out = append(out, v)
	}
	return out
}

func (a *Aggregator) Summary(window time.Duration) map[string]any {
	views := a.Query("", "", "", window)
	var pass, block, errn, fallback uint64
	open := 0
	for _, v := range views {
		if len(v.Series) == 0 {
			continue
		}
		last := v.Series[len(v.Series)-1]
		pass += last.Pass
		block += last.Block
		errn += last.Error
		fallback += last.Fallback
		if last.State == "OPEN" || last.State == "HALF_OPEN" {
			open++
		}
	}
	total := pass + block
	br, er := 0.0, 0.0
	if total > 0 {
		br = float64(block) / float64(total)
	}
	if pass > 0 {
		er = float64(errn) / float64(pass)
	}
	return map[string]any{
		"pass": pass, "block": block, "error": errn, "fallback": fallback,
		"block_ratio": br, "error_ratio": er, "circuit_open_resources": open,
		"series_count": len(views),
	}
}
