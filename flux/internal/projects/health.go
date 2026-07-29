package projects

import "os"

// HealthStatus is a coarse readiness signal for a project or source, used to
// feed the future capability Availability states (see Phase 5).
type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthAttention HealthStatus = "attention"
)

// SourceHealth reports whether a ProjectSource's local checkout is reachable.
type SourceHealth struct {
	SourceID string       `json:"sourceId"`
	Status   HealthStatus `json:"status"`
	Detail   string       `json:"detail,omitempty"`
}

// CheckSourceHealth verifies a source's resolved local path exists on disk.
// Sources with no resolvable local path (a git_repository with no checkout
// yet, an imported system) are reported as healthy — path health only
// applies to sources that claim to have local content.
func CheckSourceHealth(rootDir string, src ProjectSource) SourceHealth {
	path, err := ResolveSourcePath(rootDir, src)
	if err != nil {
		return SourceHealth{SourceID: src.ID, Status: HealthHealthy}
	}
	if _, err := os.Stat(path); err != nil {
		return SourceHealth{SourceID: src.ID, Status: HealthAttention, Detail: "path not found: " + path}
	}
	return SourceHealth{SourceID: src.ID, Status: HealthHealthy}
}
