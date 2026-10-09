package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SuperMarioYL/k3fit/internal/fit"
	"github.com/SuperMarioYL/k3fit/internal/model"
	"github.com/SuperMarioYL/k3fit/internal/quant"
	"github.com/SuperMarioYL/k3fit/internal/tps"
)

func planFor(t *testing.T, vram, ram float64) *fit.Plan {
	t.Helper()
	spec := model.DefaultK3Spec()
	tpsFn := func(q quant.Tier) float64 { return tps.Estimate(spec, q) }
	return fit.Compute(spec, vram, ram, quant.Table, tpsFn)
}

func TestRenderPlanRAMWarningAndParams(t *testing.T) {
	// Star-moment rig: the recommended Q2_K is ~835 GiB on disk vs 128 GiB RAM.
	var buf bytes.Buffer
	RenderPlan(planFor(t, 32, 128), &buf)
	out := buf.String()

	if !strings.Contains(out, "2.8T params") {
		t.Errorf("header should print the true 2.8T param count, not the rounded 3T:\n%s", out)
	}
	if !strings.Contains(out, "RAM warning:") || !strings.Contains(out, "exceed the 128 GiB RAM budget") {
		t.Errorf("expected a RAM warning when weights exceed the RAM budget:\n%s", out)
	}

	// RAM budget above the on-disk weights: the warning is silent.
	buf.Reset()
	RenderPlan(planFor(t, 32, 900), &buf)
	if strings.Contains(buf.String(), "RAM warning:") {
		t.Errorf("unexpected RAM warning when the mmap working set fits RAM:\n%s", buf.String())
	}
}

func TestRenderConstrainedRAMWarningAndParams(t *testing.T) {
	// 1M target on 96 GiB VRAM picks Q8_0 (~2608 GiB on disk) vs 256 GiB RAM.
	var buf bytes.Buffer
	RenderConstrained(planFor(t, 96, 256), 1_000_000, &buf)
	out := buf.String()

	if !strings.Contains(out, "2.8T params") {
		t.Errorf("header should print the true 2.8T param count:\n%s", out)
	}
	if !strings.Contains(out, "RAM warning:") {
		t.Errorf("expected a RAM warning for the constrained recommendation:\n%s", out)
	}
}

func TestRenderConfig(t *testing.T) {
	var buf bytes.Buffer
	RenderConfig(planFor(t, 32, 128), 0, &buf)
	out := buf.String()

	for _, want := range []string{
		"# Suggested llama.cpp launch flags",
		"--model kimi-k3-text-Q2_K.gguf",
		"--ctx-size 524288",
		"--n-gpu-layers 999",
		`--override-tensor "exps=CPU"`,
		"RAM warning:", // 835 GiB weights vs 128 GiB RAM
	} {
		if !strings.Contains(out, want) {
			t.Errorf("RenderConfig output missing %q:\n%s", want, out)
		}
	}

	// A --ctx target beyond the tier's fitted maximum is clamped, with a comment.
	buf.Reset()
	RenderConfig(planFor(t, 32, 128), 1_000_000, &buf)
	out = buf.String()
	if !strings.Contains(out, "clamped to the fitted maximum") || !strings.Contains(out, "--ctx-size 589837") {
		t.Errorf("expected the 1M target clamped to the Q2_K fit with an explanatory comment:\n%s", out)
	}

	// A --ctx target within the fit is honoured directly.
	buf.Reset()
	RenderConfig(planFor(t, 32, 128), 262144, &buf)
	if !strings.Contains(buf.String(), "--ctx-size 262144") {
		t.Errorf("expected --ctx-size 262144 for an in-fit target:\n%s", buf.String())
	}

	// Nothing fits the VRAM budget: an explicit no-emit message, not flags.
	buf.Reset()
	RenderConfig(planFor(t, 1, 1), 0, &buf)
	if !strings.Contains(buf.String(), "nothing to emit") {
		t.Errorf("expected the no-fit message:\n%s", buf.String())
	}
}

func TestSub4KFitUsesExactMaxContext(t *testing.T) {
	// --vram 18.5 leaves Q2_K's 18.49 GiB VRAM-resident weights only ~13 tokens
	// of KV headroom: a genuine fit below the smallest standard context (4K).
	// v0.2.0 rendered this as "Recommendation: Q2_K at 0 context" and
	// --emit-config emitted the unusable "--ctx-size 0".
	var buf bytes.Buffer
	RenderPlan(planFor(t, 18.5, 32), &buf)
	out := buf.String()

	if strings.Contains(out, "at 0 context") || strings.Contains(out, "VRAM at 0:") {
		t.Errorf("sub-4K fit must render the exact max context, not 0:\n%s", out)
	}
	if !strings.Contains(out, "Recommendation: Q2_K at 13 context") {
		t.Errorf("expected the exact 13-token fit in the recommendation:\n%s", out)
	}

	buf.Reset()
	RenderConfig(planFor(t, 18.5, 32), 0, &buf)
	out = buf.String()
	if strings.Contains(out, "--ctx-size 0") {
		t.Errorf("RenderConfig must never emit --ctx-size 0:\n%s", out)
	}
	if !strings.Contains(out, "--ctx-size 13") {
		t.Errorf("expected --ctx-size 13 (the exact fitted maximum):\n%s", out)
	}
}

func TestBudgetsPrintAsGiven(t *testing.T) {
	// v0.2.0 printed "Rig:  18 GiB VRAM" for --vram 18.5: %.0f rounded away the
	// budget the solver actually used. Budgets must print as given (%g); all
	// recorded integer scenarios render byte-identically (see the root
	// demo-contract guard).
	var buf bytes.Buffer
	RenderPlan(planFor(t, 18.5, 32), &buf)
	out := buf.String()
	if !strings.Contains(out, "Rig:  18.5 GiB VRAM | 32 GiB RAM") {
		t.Errorf("RenderPlan should print budgets as given (18.5), not rounded:\n%s", out)
	}
	if !strings.Contains(out, "Quant fit analysis (VRAM = 18.5 GiB)") {
		t.Errorf("quant-table header should print the budget as given:\n%s", out)
	}

	buf.Reset()
	RenderConstrained(planFor(t, 18.5, 32), 100, &buf)
	if !strings.Contains(buf.String(), "VRAM = 18.5 GiB") {
		t.Errorf("RenderConstrained header should print the budget as given:\n%s", buf.String())
	}

	buf.Reset()
	RenderConfig(planFor(t, 18.5, 32), 0, &buf)
	if !strings.Contains(buf.String(), "VRAM budget 18.5 GiB") {
		t.Errorf("RenderConfig fit basis should print the budget as given:\n%s", buf.String())
	}
}
