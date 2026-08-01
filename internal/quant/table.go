// Package quant defines the GGUF K-quant tier table used for weight sizing.
//
// Each tier carries a bits-per-weight (bpw) value — the average storage per
// parameter in the quantized GGUF file. BytesPerParam is computed at runtime
// as BPW/8 to ensure consistent floating-point evaluation across platforms
// (Go untyped-constant division rounds differently than runtime division
// for non-exact fractions like 6.1/8 or 3.27/8).
package quant

// Tier describes a single GGUF quantization tier.
type Tier struct {
	Name string  // e.g. "Q2_K"
	BPW  float64 // bits per weight
}

// BytesPerParam returns the average bytes per parameter at this tier
// (BPW / 8). Computed at runtime for cross-platform float64 consistency.
func (t Tier) BytesPerParam() float64 {
	return t.BPW / 8.0
}

// Table is the ordered list of GGUF K-quant tiers from smallest to largest.
var Table = []Tier{
	{"Q2_K", 2.5625},
	{"Q3_K_S", 2.75},
	{"Q3_K_M", 3.27},
	{"Q4_K_S", 3.5},
	{"Q4_K_M", 4.25},
	{"Q5_K_S", 5.0},
	{"Q5_K_M", 5.25},
	{"Q6_K", 6.1},
	{"Q8_0", 8.0},
	{"F16", 16.0},
}

// Lookup finds a tier by name (case-insensitive). Returns ok=false if unknown.
func Lookup(name string) (Tier, bool) {
	for _, t := range Table {
		if equalFold(t.Name, name) {
			return t, true
		}
	}
	return Tier{}, false
}

// equalFold is a minimal ASCII case-insensitive comparison.
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
