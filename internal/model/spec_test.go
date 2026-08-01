package model

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// goldenFile mirrors testdata/k3spec_golden.json.
type goldenFile struct {
	Spec struct {
		TotalLayers    int     `json:"total_layers"`
		DALayers       int     `json:"da_layers"`
		KVLayers       int     `json:"kv_layers"`
		HeadsPerLayer  int     `json:"heads_per_layer"`
		HeadDim        int     `json:"head_dim"`
		DeltaMatrixDim int     `json:"delta_matrix_dim"`
		ExpertsTotal   int     `json:"experts_total"`
		ExpertsActive  int     `json:"experts_active"`
		TotalParamsB   float64 `json:"total_params_b"`
		SharedParamsB  float64 `json:"shared_params_b"`
		ContextMax    int     `json:"context_max"`
		BytesPerElem   int     `json:"bytes_per_element"`
		ParamsPerExpB  float64 `json:"params_per_expert_b"`
		ActiveParamsB  float64 `json:"active_params_b"`
		ActivePlusSh   float64 `json:"active_plus_shared_params_b"`
	} `json:"spec"`
	Delta1M struct {
		PerHeadMatrix  int64   `json:"per_head_matrix_bytes"`
		DALayerMem     int64   `json:"da_layer_mem_bytes"`
		KVLayerMem     int64   `json:"kv_layer_mem_bytes"`
		CtxPerToken    int64   `json:"context_mem_bytes_per_token"`
		CtxMem         int64   `json:"context_mem_bytes"`
		StdKV          int64   `json:"standard_kv_bytes"`
		ReductionRatio float64 `json:"reduction_ratio"`
	} `json:"delta_at_1m"`
}

func loadGolden(t *testing.T) *goldenFile {
	t.Helper()
	// testdata is relative to the package directory.
	path := filepath.Join("..", "..", "testdata", "k3spec_golden.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var g goldenFile
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}
	return &g
}

func TestK3Spec(t *testing.T) {
	g := loadGolden(t)
	s := DefaultK3Spec()

	checks := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"TotalLayers", s.TotalLayers, g.Spec.TotalLayers},
		{"DALayers", s.DALayers, g.Spec.DALayers},
		{"KVLayers", s.KVLayers, g.Spec.KVLayers},
		{"HeadsPerLayer", s.HeadsPerLayer, g.Spec.HeadsPerLayer},
		{"HeadDim", s.HeadDim, g.Spec.HeadDim},
		{"DeltaMatrixDim", s.DeltaMatrixDim, g.Spec.DeltaMatrixDim},
		{"ExpertsTotal", s.ExpertsTotal, g.Spec.ExpertsTotal},
		{"ExpertsActive", s.ExpertsActive, g.Spec.ExpertsActive},
		{"TotalParamsB", s.TotalParamsB, g.Spec.TotalParamsB},
		{"SharedParamsB", s.SharedParamsB, g.Spec.SharedParamsB},
		{"ContextMax", s.ContextMax, g.Spec.ContextMax},
		{"BytesPerElement", s.BytesPerElement, g.Spec.BytesPerElem},
		{"ParamsPerExpertB", s.ParamsPerExpertB(), g.Spec.ParamsPerExpB},
		{"ActiveParamsB", s.ActiveParamsB(), g.Spec.ActiveParamsB},
		{"ActivePlusSharedParamsB", s.ActivePlusSharedParamsB(), g.Spec.ActivePlusSh},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestDeltaMemAccount1M(t *testing.T) {
	g := loadGolden(t)
	s := DefaultK3Spec()
	d := ComputeDeltaMemAccount(s, s.ContextMax)

	checks := []struct {
		name string
		got  int64
		want int64
	}{
		{"PerHeadMatrixBytes", d.PerHeadMatrixBytes, g.Delta1M.PerHeadMatrix},
		{"DALayerMemBytes", d.DALayerMemBytes, g.Delta1M.DALayerMem},
		{"KVLayerMemBytes", d.KVLayerMemBytes, g.Delta1M.KVLayerMem},
		{"ContextMemBytesPerToken", d.ContextMemBytesPerToken, g.Delta1M.CtxPerToken},
		{"ContextMemBytes", d.ContextMemBytes, g.Delta1M.CtxMem},
		{"StandardKVBytes", d.StandardKVBytes, g.Delta1M.StdKV},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, c.got, c.want)
		}
	}

	// Reduction ratio (floating-point, use tolerance).
	ratio := d.ReductionRatio()
	if ratio < g.Delta1M.ReductionRatio-0.001 || ratio > g.Delta1M.ReductionRatio+0.001 {
		t.Errorf("ReductionRatio: got %f, want %f (±0.001)", ratio, g.Delta1M.ReductionRatio)
	}
}

func TestDeltaMemAccountContextLinear(t *testing.T) {
	s := DefaultK3Spec()
	d1M := ComputeDeltaMemAccount(s, 1_000_000)
	d512K := ComputeDeltaMemAccount(s, 524288)

	// DA matrix is constant; KV scales linearly. 512K is ~half of 1M.
	daConst := d1M.DALayerMemBytes
	if d512K.DALayerMemBytes != daConst {
		t.Errorf("DA matrix should be constant: 1M=%d, 512K=%d", daConst, d512K.DALayerMemBytes)
	}
	// KV at 512K should be ~half of 1M (512000/1000000 of 1M).
	ratio := float64(d512K.KVLayerMemBytes) / float64(d1M.KVLayerMemBytes)
	if ratio < 0.523 || ratio > 0.525 { // 524288/1000000 = 0.524288
		t.Errorf("KV ratio 512K/1M: got %f, want ~0.5243", ratio)
	}
}

func TestExpertActivationRatio(t *testing.T) {
	s := DefaultK3Spec()
	ratio := s.ExpertActivationRatio()
	want := 16.0 / 896.0
	if ratio < want-0.0001 || ratio > want+0.0001 {
		t.Errorf("ExpertActivationRatio: got %f, want %f", ratio, want)
	}
	if ratio*100 < 1.5 || ratio*100 > 2.0 {
		t.Errorf("Expected ~1.8%% activation, got %.1f%%", ratio*100)
	}
}
