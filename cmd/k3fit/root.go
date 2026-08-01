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
	vramGiB float64
	ramGiB  float64
	ctxFlag int
	quantF  string
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
  k3fit --vram 32 --ram 128 --ctx 512000    # check a specific context size`,
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
	_ = rootCmd.MarkFlagRequired("vram")
	_ = rootCmd.MarkFlagRequired("ram")
}

func runRoot(cmd *cobra.Command, args []string) error {
	spec := model.DefaultK3Spec()

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

	// If --ctx is set, annotate the recommended context.
	if ctxFlag > 0 {
		report.RenderConstrained(plan, ctxFlag, os.Stdout)
		return nil
	}

	report.RenderPlan(plan, os.Stdout)
	return nil
}
