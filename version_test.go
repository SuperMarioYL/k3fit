package k3fit

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func TestVersionShape(t *testing.T) {
	if !semverRe.MatchString(Version) {
		t.Fatalf("Version %q is not bare semver x.y.z", Version)
	}
}

// TestVersionLockstep asserts every published version surface agrees with the
// VERSION file — the single source embedded in this package. A future bump
// that misses a surface fails here instead of drifting.
func TestVersionLockstep(t *testing.T) {
	v := Version

	for _, readme := range []string{"README.md", "README.zh-CN.md"} {
		b, err := os.ReadFile(readme)
		if err != nil {
			t.Fatalf("read %s: %v", readme, err)
		}
		if !strings.Contains(string(b), "`v"+v+"`") {
			t.Errorf("%s: no `v%s` version badge", readme, v)
		}
	}

	siteRaw, err := os.ReadFile("web/site.json")
	if err != nil {
		t.Fatalf("read web/site.json: %v", err)
	}
	var site struct {
		Meta struct {
			ContentVersion string `json:"content_version"`
		} `json:"meta"`
		Footer struct {
			Tag string `json:"tag"`
		} `json:"footer"`
	}
	if err := json.Unmarshal(siteRaw, &site); err != nil {
		t.Fatalf("parse web/site.json: %v", err)
	}
	if got := strings.TrimPrefix(site.Meta.ContentVersion, "v"); got != v {
		t.Errorf("web/site.json meta.content_version = %q, want %q", site.Meta.ContentVersion, v)
	}
	if !strings.HasPrefix(site.Footer.Tag, v+" ") {
		t.Errorf("web/site.json footer.tag = %q, want prefix %q", site.Footer.Tag, v+" ")
	}

	demoRaw, err := os.ReadFile("docs/demo-results.json")
	if err != nil {
		t.Fatalf("read docs/demo-results.json: %v", err)
	}
	var demo struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(demoRaw, &demo); err != nil {
		t.Fatalf("parse docs/demo-results.json: %v", err)
	}
	if got := strings.TrimPrefix(demo.Version, "v"); got != v {
		t.Errorf("docs/demo-results.json version = %q, want %q", demo.Version, v)
	}

	changelog, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatalf("read CHANGELOG.md: %v", err)
	}
	if !strings.Contains(string(changelog), "## [v"+v+"]") {
		t.Errorf("CHANGELOG.md has no \"## [v%s]\" heading", v)
	}
}
