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
