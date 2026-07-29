package projects

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// ProjectEventType categorizes a timeline entry.
type ProjectEventType string

const (
	EventProjectCreated  ProjectEventType = "project_created"
	EventProjectUpdated  ProjectEventType = "project_updated"
	EventProjectArchived ProjectEventType = "project_archived"
	EventSourceAdded     ProjectEventType = "source_added"
	EventSourceRemoved   ProjectEventType = "source_removed"
	EventMigrated        ProjectEventType = "migrated_from_workspace"
)

// ProjectEvent is one entry in a Project's activity timeline. Metadata must
// be filtered by callers (e.g. via internal/masker) before recording so
// secret values never enter event storage.
type ProjectEvent struct {
	ID         string            `json:"id"`
	ProjectID  string            `json:"projectId"`
	Type       ProjectEventType  `json:"type"`
	Summary    string            `json:"summary"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	OccurredAt string            `json:"occurredAt"`
}

const activityLogFile = "activity.jsonl"

// RecordEvent appends an event to a project's local activity log
// (.reqit/state/activity.jsonl). Activity is local state, not portable
// manifest data, so it is never synced or committed.
func RecordEvent(rootDir string, evt ProjectEvent) error {
	if evt.ID == "" {
		evt.ID = uuid.NewString()
	}
	if evt.OccurredAt == "" {
		evt.OccurredAt = now()
	}
	line, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(StateDir(rootDir), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(StateDir(rootDir), activityLogFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}
