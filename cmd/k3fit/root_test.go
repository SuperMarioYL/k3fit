package main

import (
	"strings"
	"testing"

	"github.com/SuperMarioYL/k3fit/internal/model"
)

func TestValidateFlags(t *testing.T) {
	spec := model.DefaultK3Spec()

	cases := []struct {
		name      string
		vram, ram float64
		ctx       int
		wantErr   string // empty = expect success
	}{
		{"valid full", 32, 128, 0, ""},
		{"valid ctx target", 32, 128, 512000, ""},
		{"valid ctx at model max", 96, 256, spec.ContextMax, ""},
		{"zero vram", 0, 128, 0, "--vram"},
		{"negative vram", -1, 128, 0, "--vram"},
		{"zero ram", 32, 0, 0, "--ram"},
		{"negative ram", 32, -8, 0, "--ram"},
		{"negative ctx silently ignored before", 32, 128, -5, "--ctx"},
		{"ctx beyond model max", 96, 256, spec.ContextMax + 1, "exceeds the Kimi K3 maximum"},
	}
	for _, c := range cases {
		err := validateFlags(spec, c.vram, c.ram, c.ctx)
		if c.wantErr == "" {
			if err != nil {
				t.Errorf("%s: unexpected error: %v", c.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("%s: want error containing %q, got %v", c.name, c.wantErr, err)
		}
	}
}
