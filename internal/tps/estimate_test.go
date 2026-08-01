package tps

import (
	"testing"

	"github.com/SuperMarioYL/k3fit/internal/model"
	"github.com/SuperMarioYL/k3fit/internal/quant"
)

func TestEstimateQ2K(t *testing.T) {
	spec := model.DefaultK3Spec()
	q, ok := quant.Lookup("Q2_K")
	if !ok {
		t.Fatal("Q2_K not found")
	}
	tps := Estimate(spec, q)
	// Q2_K: active_plus_shared = 62B, bpp = 0.3203125
	// activeGB = 62 * 0.3203125 = 19.859375
	// tps = 240 / 19.859375 ≈ 12.085
	if tps < 12.0 || tps > 12.2 {
		t.Errorf("Q2_K tps: got %f, want ~12.08", tps)
	}
}

func TestEstimateMonotonic(t *testing.T) {
	spec := model.DefaultK3Spec()
	// Higher quant → more bytes per param → lower tps.
	prev := 0.0
	for _, q := range quant.Table {
		v := Estimate(spec, q)
		if prev > 0 && v > prev {
			t.Errorf("tps should decrease with larger quant: %s=%f > prev %f", q.Name, v, prev)
		}
		prev = v
	}
}

func TestRange(t *testing.T) {
	low, high := Range(12.0)
	// ±30%: low = 8.4, high = 15.6
	if low < 8.3 || low > 8.5 {
		t.Errorf("low: got %f, want ~8.4", low)
	}
	if high < 15.5 || high > 15.7 {
		t.Errorf("high: got %f, want ~15.6", high)
	}
}
