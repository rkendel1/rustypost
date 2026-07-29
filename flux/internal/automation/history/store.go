package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Entry struct {
	ID            string         `json:"id"`
	Kind          string         `json:"kind"`
	Name          string         `json:"name"`
	Status        string         `json:"status"`
	CorrelationID string         `json:"correlationId,omitempty"`
	WorkspaceID   string         `json:"workspaceId,omitempty"`
	RepositoryID  string         `json:"repositoryId,omitempty"`
	JobID         string         `json:"jobId,omitempty"`
	PipelineID    string         `json:"pipelineId,omitempty"`
	StartedAt     time.Time      `json:"startedAt"`
	FinishedAt    time.Time      `json:"finishedAt"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(workspaceRoot string) *Store {
	return &Store{path: filepath.Join(workspaceRoot, ".reqit", "automation", "history.json")}
}

func (s *Store) Append(entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.readLocked()
	if err != nil {
		return err
	}
	if entry.FinishedAt.IsZero() {
		entry.FinishedAt = time.Now().UTC()
	}
	if entry.StartedAt.IsZero() {
		entry.StartedAt = entry.FinishedAt
	}
	rows = append(rows, entry)
	return s.writeLocked(rows)
}

func (s *Store) List(limit int) ([]Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.readLocked()
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[len(rows)-limit:]
	}
	out := make([]Entry, len(rows))
	copy(out, rows)
	return out, nil
}

func (s *Store) readLocked() ([]Entry, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, err
	}
	var rows []Entry
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) writeLocked(rows []Entry) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}
