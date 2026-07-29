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
	ProjectID     string         `json:"projectId,omitempty"`
	SourceID      string         `json:"sourceId,omitempty"`
	JobID         string         `json:"jobId,omitempty"`
	PipelineID    string         `json:"pipelineId,omitempty"`
	StartedAt     time.Time      `json:"startedAt"`
	FinishedAt    time.Time      `json:"finishedAt"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// Redactor masks secret-shaped substrings out of free-text fields before
// they're persisted. internal/masker.Engine implements this.
type Redactor interface {
	Mask(string) string
}

type Store struct {
	path     string
	mu       sync.Mutex
	redactor Redactor
}

func NewStore(workspaceRoot string) *Store {
	return &Store{path: filepath.Join(workspaceRoot, ".reqit", "automation", "history.json")}
}

// SetRedactor installs a Redactor applied to Name and any string Metadata
// values before an entry is appended. Optional — if unset, entries are
// persisted as given.
func (s *Store) SetRedactor(r Redactor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.redactor = r
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
	if s.redactor != nil {
		entry.Name = s.redactor.Mask(entry.Name)
		if entry.Metadata != nil {
			redacted := make(map[string]any, len(entry.Metadata))
			for k, v := range entry.Metadata {
				if str, ok := v.(string); ok {
					redacted[k] = s.redactor.Mask(str)
				} else {
					redacted[k] = v
				}
			}
			entry.Metadata = redacted
		}
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
