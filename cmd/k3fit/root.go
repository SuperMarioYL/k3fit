package main

import (
	"fmt"
	"os"

	"github.com/SuperMarioYL/k3fit/internal/fit"
	"github.com/SuperMarioYL/k3fit/internal/model"
	"github.com/SuperMarioYL/k3fit/internal/quant"
	"github.com/SuperMarioYL/k3fit/internal/report"
	"github.com/SuperMarioYL/k3fit/internal/tps"
	"github.com/spf13/cobra"
)

var (
	vramGiB    float64
	ramGiB     float64
	ctxFlag    int
	quantF     string
	emitConfig bool
)

var rootCmd = &cobra.Command{
	Use:   "k3fit",
	Short: "Size Kimi K3 for your rig before downloading 1.4TB",
	Long: `K3Fit is a one-command Go CLI that uses Kimi K3's Delta-Attention memory
model to pick your max-context, GGUF quant, and expert-routing — and reports
a predicted tps before you commit to a 1.4TB download, so your Agent workflows
and homelab rigs are sized correctly the first time.

No model download. No GPU touch. Pure arithmetic over the K3 architecture spec.

Usage:
  k3fit --vram 32 --ram 128              # full fit + quant + routing + tps
  k3fit --vram 96 --ram 256 --quant Q3_K_M  # constrained to a specific quant
  k3fit --vram 32 --ram 128 --ctx 512000    # check a specific context size
  k3fit --vram 32 --ram 128 --emit-config   # suggested llama.cpp launch flags`,
	RunE: runRoot,
}

func init() {
	rootCmd.Flags().Float64VarP(&vramGiB, "vram", "v", 0,
		"VRAM budget in GiB (required)")
	rootCmd.Flags().Float64VarP(&ramGiB, "ram", "r", 0,
		"RAM budget in GiB (required)")
	rootCmd.Flags().IntVar(&ctxFlag, "ctx", 0,
		"target context length in tokens (0 = auto-pick max)")
	rootCmd.Flags().StringVarP(&quantF, "quant", "q", "",
		"constrain to a specific quant tier (e.g. Q2_K, Q3_K_M, Q4_K_M)")
	rootCmd.Flags().BoolVar(&emitConfig, "emit-config", false,
		"print suggested llama.cpp launch flags for the recommended fit")
	_ = rootCmd.MarkFlagRequired("vram")
	_ = rootCmd.MarkFlagRequired("ram")
}

// validateFlags rejects nonsense budgets up front instead of letting them flow
// into the planner's byte math: zero/negative VRAM or RAM produce a negative
// budget, and a --ctx beyond the K3 maximum would report a fit the model
// cannot run.
func validateFlags(spec model.K3Spec, vram, ram float64, ctx int) error {
	if vram <= 0 {
		return fmt.Errorf("--vram must be > 0 GiB (got %g)", vram)
	}
	if ram <= 0 {
		return fmt.Errorf("--ram must be > 0 GiB (got %g)", ram)
	}
	if ctx < 0 {
		return fmt.Errorf("--ctx must be >= 0 tokens, 0 = auto-pick max (got %d)", ctx)
	}
	if ctx > spec.ContextMax {
		return fmt.Errorf("--ctx %d exceeds the Kimi K3 maximum of %d tokens", ctx, spec.ContextMax)
	}
	return nil
}

func runRoot(cmd *cobra.Command, args []string) error {
	spec := model.DefaultK3Spec()

	if err := validateFlags(spec, vramGiB, ramGiB, ctxFlag); err != nil {
		return err
	}

	// Select quant tiers.
	quants := quant.Table
	if quantF != "" {
		t, ok := quant.Lookup(quantF)
		if !ok {
			return fmt.Errorf("unknown quant tier %q (try Q2_K, Q3_K_M, Q4_K_M, …)", quantF)
		}
		quants = []quant.Tier{t}
	}

	tpsFn := func(q quant.Tier) float64 {
		return tps.Estimate(spec, q)
	}

	plan := fit.Compute(spec, vramGiB, ramGiB, quants, tpsFn)

	// Suggested launch flags for the recommended fit.
	if emitConfig {
		report.RenderConfig(plan, ctxFlag, os.Stdout)
		return nil
	}

	// If --ctx is set, annotate the recommended context.
	if ctxFlag > 0 {
		report.RenderConstrained(plan, ctxFlag, os.Stdout)
		return nil
	}

	report.RenderPlan(plan, os.Stdout)
	return nil
}
