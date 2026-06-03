package check

import "github.com/bantuson/beekeeper/internal/catalog"

// resolveCatalogHealthy delegates to catalog.ResolveHealthy, the single shared
// implementation for all caller-tier packages (check, gateway, watch, scan).
//
// See internal/catalog/health.go for the full rationale, security note, and
// fail-safe documentation (WR-01: single source of truth; WR-04: deliberate
// healthy=true default on read failure).
func resolveCatalogHealthy(cacheDir string) bool {
	return catalog.ResolveHealthy(cacheDir)
}
