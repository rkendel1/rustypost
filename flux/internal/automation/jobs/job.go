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
	StatusPaused    Status = "paused"
	StatusCancelled Status = "cancelled"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type Job struct {
	ID            string    `json:"id"`
	Type          Type      `json:"type"`
	Status        Status    `json:"status"`
	Progress      int       `json:"progress"`
	MaxRetries    int       `json:"maxRetries,omitempty"`
	Attempts      int       `json:"attempts,omitempty"`
	CorrelationID string    `json:"correlationId,omitempty"`
	ProjectID     string    `json:"projectId"`
	SourceID      string    `json:"sourceId,omitempty"`
	PipelineID    string    `json:"pipelineId,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	StartedAt     time.Time `json:"startedAt,omitempty"`
	UpdatedAt     time.Time `json:"updatedAt"`
	FinishedAt    time.Time `json:"finishedAt,omitempty"`
	Payload       any       `json:"payload,omitempty"`
	Result        any       `json:"result,omitempty"`
	Error         string    `json:"error,omitempty"`
}
