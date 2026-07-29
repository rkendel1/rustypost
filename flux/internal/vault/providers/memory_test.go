package providers

import "testing"

func TestMemoryProviderRoundTrip(t *testing.T) {
	p := NewMemoryProvider()
	if err := p.Set("id1", "value"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := p.Get("id1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "value" {
		t.Errorf("expected value, got %q", got)
	}
	if err := p.Delete("id1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := p.Get("id1"); err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}
