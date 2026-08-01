package fit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/SuperMarioYL/k3fit/internal/model"
	"github.com/SuperMarioYL/k3fit/internal/quant"
	"github.com/SuperMarioYL/k3fit/internal/tps"
)

type goldenFit struct {
	Quant           string  `json:"quant"`
	BPW             float64 `json:"bpw"`
	BytesPerParam   float64 `json:"bytes_per_param"`
	TotalWeightsB   int64   `json:"total_weights_bytes"`
	ActiveExpertB   int64   `json:"active_expert_bytes"`
	SharedB         int64   `json:"shared_bytes"`
	MaxContext      int     `json:"max_context"`
	StandardCtx     int     `json:"standard_ctx"`
	Fits1M          bool    `json:"fits_1m"`
	TPS             float64 `json:"tps"`
}

type goldenRoot struct {
	Fit        []goldenFit `json:"fit_32_128"`
	StarMoment struct {
		VRAMGiB            float64 `json:"vram_gib"`
		RAMGiB             float64 `json:"ram_gib"`
		OneMFitsAnyQuant   bool    `json:"one_m_fits_any_quant"`
		RecommendedQuant   string  `json:"recommended_quant"`
		RecommendedStdCtx  int     `json:"recommended_standard_ctx"`
		RecommendedTPS     float64 `json:"recommended_tps"`
	} `json:"star_moment"`
}

func loadGoldenRoot(t *testing.T) *goldenRoot {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "k3spec_golden.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var g goldenRoot
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}
	return &g
}

func TestPlanStarMoment(t *testing.T) {
	g := loadGoldenRoot(t)
	spec := model.DefaultK3Spec()

	tpsFn := func(q quant.Tier) float64 { return tps.Estimate(spec, q) }
	plan := Compute(spec, g.StarMoment.VRAMGiB, g.StarMoment.RAMGiB, quant.Table, tpsFn)

	// 1M must NOT fit at any quant.
	for _, r := range plan.Results {
		if r.Fits1M {
			t.Errorf("quant %s: 1M should not fit in %.0f GiB VRAM", r.Quant.Name, g.StarMoment.VRAMGiB)
		}
	}
	if g.StarMoment.OneMFitsAnyQuant {
		t.Fatal("golden says 1M fits any quant — expected false")
	}

	// Recommended quant must be Q2_K with 512K standard ctx.
	if plan.Recommended == nil {
		t.Fatal("expected a recommendation")
	}
	if plan.Recommended.Quant.Name != g.StarMoment.RecommendedQuant {
		t.Errorf("recommended quant: got %s, want %s", plan.Recommended.Quant.Name, g.StarMoment.RecommendedQuant)
	}
	if plan.Recommended.StandardCtx != g.StarMoment.RecommendedStdCtx {
		t.Errorf("recommended std ctx: got %d, want %d", plan.Recommended.StandardCtx, g.StarMoment.RecommendedStdCtx)
	}
}

func TestPlanPerQuantGolden(t *testing.T) {
	g := loadGoldenRoot(t)
	spec := model.DefaultK3Spec()

	tpsFn := func(q quant.Tier) float64 { return tps.Estimate(spec, q) }
	plan := Compute(spec, 32, 128, quant.Table, tpsFn)

	if len(plan.Results) != len(g.Fit) {
		t.Fatalf("result count: got %d, want %d", len(plan.Results), len(g.Fit))
	}

	for i, r := range plan.Results {
		gf := g.Fit[i]
		if r.Quant.Name != gf.Quant {
			t.Errorf("[%d] quant name: got %s, want %s", i, r.Quant.Name, gf.Quant)
		}
		if r.MaxContext != gf.MaxContext {
			t.Errorf("[%d] %s max_context: got %d, want %d", i, r.Quant.Name, r.MaxContext, gf.MaxContext)
		}
		if r.StandardCtx != gf.StandardCtx {
			t.Errorf("[%d] %s standard_ctx: got %d, want %d", i, r.Quant.Name, r.StandardCtx, gf.StandardCtx)
		}
		if r.Fits1M != gf.Fits1M {
			t.Errorf("[%d] %s fits_1m: got %v, want %v", i, r.Quant.Name, r.Fits1M, gf.Fits1M)
		}
		// TPS: allow small float tolerance.
		if r.TPS < gf.TPS-0.01 || r.TPS > gf.TPS+0.01 {
			t.Errorf("[%d] %s tps: got %f, want %f", i, r.Quant.Name, r.TPS, gf.TPS)
		}
	}
}

func TestLargestStandardCtx(t *testing.T) {
	cases := []struct {
		n    int
		want int
	}{
		{589837, 524288},
		{530709, 524288},
		{366728, 262144},
		{57687, 32768},
		{0, 0},
		{500, 0},
		{1000000, 1000000},
	}
	for _, c := range cases {
		got := largestStandardCtx(c.n)
		if got != c.want {
			t.Errorf("largestStandardCtx(%d): got %d, want %d", c.n, got, c.want)
		}
	}
}
