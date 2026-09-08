# Changelog

All notable changes to K3Fit are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions match the
VERSION file and the GitHub release tags.

## [v0.2.0] - 2026-09-09

### Fixed

- The `--ram` budget is now enforced as a warning gate: tiers whose on-disk
  weights exceed the RAM budget are flagged in the report, because the mmap
  working set pages from disk and the RAM-bandwidth-bound tps estimate no
  longer holds (`internal/fit/planner.go`, `internal/report/render.go`).
- Invalid budgets are rejected up front: `--vram`/`--ram` must be positive and
  `--ctx` must stay within the K3 maximum of 1,000,000 tokens instead of being
  silently ignored or reporting a fit the model cannot run
  (`cmd/k3fit/root.go`).
- The report header prints the true `2.8T` parameter count instead of the
  rounded `3T` (`internal/report/render.go`).

### Added

- `--emit-config` prints suggested llama.cpp launch flags (model file, clamped
  `--ctx-size`, expert tensors pinned to CPU) for the recommended fit,
  completing plan milestone m3.

### Changed

- The version is owned by the `VERSION` file via `go:embed` and held in
  lockstep across the CLI, both READMEs, the site and the demo records by a
  test; this changelog starts the contiguous history.

## [v0.1.0] - 2026-08-02

- Initial release: Delta-Attention memory account (69 DA + 24 KV layers),
  per-quant max-context fit, bandwidth-bound heuristic tps with a ±30% band,
  `--quant`/`--ctx` constraint flags, bilingual README and product site.
