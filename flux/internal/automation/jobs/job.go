package jobs

import "time"

type Type string

const (
	TypeRepositoryScan      Type = "repository_scan"
	TypeGenerateOpenAPI     Type = "generate_openapi"
	TypeGenerateCollections Type = "generate_collections"
	TypeGenerateTests       Type = "generate_tests"
	TypeInstallCI           Type = "install_ci"
	TypeCreatePR            Type = "create_pr"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type Job struct {
	ID        string    `json:"id"`
	Type      Type      `json:"type"`
	Status    Status    `json:"status"`
	Progress  int       `json:"progress"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Payload   any       `json:"payload,omitempty"`
	Result    any       `json:"result,omitempty"`
	Error     string    `json:"error,omitempty"`
}
