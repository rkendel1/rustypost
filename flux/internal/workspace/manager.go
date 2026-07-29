package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	metadataDirName = ".reqit-workspace"
	cacheDirName    = "cache"
)

type Info struct {
	Name       string `json:"name"`
	Repository string `json:"repository"`
	Branch     string `json:"branch"`
	Framework  string `json:"framework"`
	Endpoints  int    `json:"endpoints"`
	Status     string `json:"status"`
	UpdatedAt  string `json:"updatedAt"`
}

type ScanHistory struct {
	At           string `json:"at"`
	FilesScanned int    `json:"filesScanned"`
	Endpoints    int    `json:"endpoints"`
}

type Manager struct {
	root string
}

func NewManager(root string) *Manager {
	return &Manager{root: root}
}

func (m *Manager) Root() string {
	return m.root
}

func (m *Manager) MetadataDir() string {
	return filepath.Join(m.root, metadataDirName)
}

func (m *Manager) CacheDir() string {
	return filepath.Join(m.MetadataDir(), cacheDirName)
}

func (m *Manager) Ensure() error {
	if err := os.MkdirAll(m.CacheDir(), 0o755); err != nil {
		return err
	}
	return nil
}

func (m *Manager) LoadInfo() (Info, error) {
	var info Info
	return info, loadJSON(filepath.Join(m.MetadataDir(), "workspace.json"), &info)
}

func (m *Manager) SaveInfo(info Info) error {
	if info.UpdatedAt == "" {
		info.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := m.Ensure(); err != nil {
		return err
	}
	return writeJSON(filepath.Join(m.MetadataDir(), "workspace.json"), info)
}

func (m *Manager) LoadScanHistory() ([]ScanHistory, error) {
	var rows []ScanHistory
	if err := loadJSON(filepath.Join(m.MetadataDir(), "scan-history.json"), &rows); err != nil {
		if os.IsNotExist(err) {
			return []ScanHistory{}, nil
		}
		return nil, err
	}
	return rows, nil
}

func (m *Manager) AppendScanHistory(row ScanHistory, max int) error {
	if row.At == "" {
		row.At = time.Now().UTC().Format(time.RFC3339)
	}
	rows, err := m.LoadScanHistory()
	if err != nil {
		return err
	}
	rows = append(rows, row)
	if max > 0 && len(rows) > max {
		rows = rows[len(rows)-max:]
	}
	if err := m.Ensure(); err != nil {
		return err
	}
	return writeJSON(filepath.Join(m.MetadataDir(), "scan-history.json"), rows)
}

func (m *Manager) LoadSettings() (map[string]any, error) {
	settings := map[string]any{}
	if err := loadJSON(filepath.Join(m.MetadataDir(), "settings.json"), &settings); err != nil {
		if os.IsNotExist(err) {
			return settings, nil
		}
		return nil, err
	}
	return settings, nil
}

func (m *Manager) SaveSettings(settings map[string]any) error {
	if settings == nil {
		settings = map[string]any{}
	}
	if err := m.Ensure(); err != nil {
		return err
	}
	return writeJSON(filepath.Join(m.MetadataDir(), "settings.json"), settings)
}

func (m *Manager) WriteCache(name string, data any) error {
	if err := m.Ensure(); err != nil {
		return err
	}
	return writeJSON(filepath.Join(m.CacheDir(), name), data)
}

func (m *Manager) ReadCache(name string, out any) error {
	return loadJSON(filepath.Join(m.CacheDir(), name), out)
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func loadJSON(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}
