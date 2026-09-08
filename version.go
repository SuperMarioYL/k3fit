// Package k3fit is the K3Fit module root. Its single responsibility is owning
// the version: the VERSION file at the repository root is the one authoritative
// value, embedded at build time so `go install`-ed binaries report it without a
// runtime file read. Every other surface — CLI, READMEs, web/site.json, demo
// records, CHANGELOG — is held in lockstep against this value by version_test.go.
package k3fit

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var versionRaw string

// Version is the current K3Fit version (VERSION file contents, trimmed).
var Version = strings.TrimSpace(versionRaw)
