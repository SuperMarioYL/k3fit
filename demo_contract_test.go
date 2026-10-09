package k3fit

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/SuperMarioYL/k3fit/internal/fit"
	"github.com/SuperMarioYL/k3fit/internal/model"
	"github.com/SuperMarioYL/k3fit/internal/quant"
	"github.com/SuperMarioYL/k3fit/internal/report"
	"github.com/SuperMarioYL/k3fit/internal/tps"
)

type demoRecord struct {
	Steps []struct {
		Command string `json:"command"`
		Output  string `json:"output"`
	} `json:"steps"`
}

// recordedScenarios mirrors examples/presentation_demo.sh: the exact budgets and
// quant filter of the two recorded demo scenarios.
var recordedScenarios = []struct {
	vram, ram float64
	quants    []quant.Tier
}{
	{32, 128, quant.Table},
	{96, 256, []quant.Tier{mustTierQ3KM()}},
}

func mustTierQ3KM() quant.Tier {
	q, ok := quant.Lookup("Q3_K_M")
	if !ok {
		panic("Q3_K_M missing from the quant table")
	}
	return q
}

// renderRecorded re-runs the recorded demo scenarios in-process and returns
// exactly what the CLI prints for each.
func renderRecorded(t *testing.T) []string {
	t.Helper()
	spec := model.DefaultK3Spec()
	tpsFn := func(q quant.Tier) float64 { return tps.Estimate(spec, q) }

	outputs := make([]string, len(recordedScenarios))
	for i, s := range recordedScenarios {
		plan := fit.Compute(spec, s.vram, s.ram, s.quants, tpsFn)
		var buf bytes.Buffer
		report.RenderPlan(plan, &buf)
		outputs[i] = buf.String()
	}
	return outputs
}

// TestDemoRecordedOutputContract pins the recorded CLI output in every published
// surface: docs/demo-results.json (byte-equal), web/site.json (byte-equal) and
// the README sample blocks (contained). A report change that forgets one of
// these surfaces fails here instead of shipping a README-vs-code contradiction.
func TestDemoRecordedOutputContract(t *testing.T) {
	outputs := renderRecorded(t)

	demoRaw, err := os.ReadFile("docs/demo-results.json")
	if err != nil {
		t.Fatalf("read docs/demo-results.json: %v", err)
	}
	var demo demoRecord
	if err := json.Unmarshal(demoRaw, &demo); err != nil {
		t.Fatalf("parse docs/demo-results.json: %v", err)
	}
	if len(demo.Steps) != len(outputs) {
		t.Fatalf("demo-results steps: got %d, want %d", len(demo.Steps), len(outputs))
	}
	for i, out := range outputs {
		if demo.Steps[i].Output != out {
			t.Errorf("docs/demo-results.json step %d (%s) drifted from the real CLI output — regenerate the demo records in lockstep",
				i, demo.Steps[i].Command)
		}
	}

	siteRaw, err := os.ReadFile("web/site.json")
	if err != nil {
		t.Fatalf("read web/site.json: %v", err)
	}
	var site struct {
		Demo struct {
			Steps []struct {
				Command string `json:"command"`
				Output  string `json:"output"`
			} `json:"steps"`
		} `json:"demo"`
	}
	if err := json.Unmarshal(siteRaw, &site); err != nil {
		t.Fatalf("parse web/site.json: %v", err)
	}
	if len(site.Demo.Steps) != len(outputs) {
		t.Fatalf("site demo steps: got %d, want %d", len(site.Demo.Steps), len(outputs))
	}
	for i, out := range outputs {
		if site.Demo.Steps[i].Output != out {
			t.Errorf("web/site.json demo step %d (%s) drifted from the real CLI output — regenerate the demo records in lockstep",
				i, site.Demo.Steps[i].Command)
		}
	}

	for _, readme := range []string{"README.md", "README.zh-CN.md"} {
		b, err := os.ReadFile(readme)
		if err != nil {
			t.Fatalf("read %s: %v", readme, err)
		}
		for i, out := range outputs {
			if !strings.Contains(string(b), strings.TrimSpace(out)) {
				t.Errorf("%s: sample block %d does not contain the real CLI output", readme, i)
			}
		}
	}
}
