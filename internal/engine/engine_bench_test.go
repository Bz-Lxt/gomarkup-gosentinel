package engine

import (
	"testing"

	"gosentinel/internal/rule"
)

func BenchmarkEnterExit(b *testing.B) {
	e := New(nil)
	r := rule.Default()
	r.Service, r.Resource, r.QPS = "svc", "hot", 1_000_000
	e.SetRule(r)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		en := e.Enter("svc", "hot", "*")
		en.Exit(FinishOK)
	}
}
