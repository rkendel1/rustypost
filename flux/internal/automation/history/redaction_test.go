package history

import "testing"

type maskingRedactor struct{}

func (maskingRedactor) Mask(s string) string { return "[REDACTED]" }

func TestStoreRedactsNameAndStringMetadataOnAppend(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	s.SetRedactor(maskingRedactor{})

	if err := s.Append(Entry{
		ID:     "e1",
		Kind:   "job",
		Name:   "ghp_realSecretValue",
		Status: "failed",
		Metadata: map[string]any{
			"error": "token=ghp_anotherSecret",
			"count": 3,
		},
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	rows, err := s.List(0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(rows))
	}
	if rows[0].Name != "[REDACTED]" {
		t.Errorf("expected redacted Name, got %q", rows[0].Name)
	}
	if rows[0].Metadata["error"] != "[REDACTED]" {
		t.Errorf("expected redacted string metadata, got %v", rows[0].Metadata["error"])
	}
	if rows[0].Metadata["count"] != float64(3) { // round-tripped through JSON as float64
		t.Errorf("expected non-string metadata to survive untouched, got %v", rows[0].Metadata["count"])
	}
}

func TestStoreWithoutRedactorPassesThrough(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)

	if err := s.Append(Entry{ID: "e1", Name: "plain-name"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	rows, err := s.List(0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if rows[0].Name != "plain-name" {
		t.Errorf("expected unredacted name without a redactor, got %q", rows[0].Name)
	}
}
