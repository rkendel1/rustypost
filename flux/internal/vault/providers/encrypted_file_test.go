package providers

import (
	"os"
	"testing"
)

func TestEncryptedFileRequiresConsentBeforeUse(t *testing.T) {
	p := NewEncryptedFileProvider(t.TempDir())
	if err := p.Set("id1", "value"); err != ErrConsentRequired {
		t.Errorf("expected ErrConsentRequired before Unlock, got %v", err)
	}
	if _, err := p.Get("id1"); err != ErrConsentRequired {
		t.Errorf("expected ErrConsentRequired before Unlock, got %v", err)
	}
	if err := p.Delete("id1"); err != ErrConsentRequired {
		t.Errorf("expected ErrConsentRequired before Unlock, got %v", err)
	}
}

func TestEncryptedFileRoundTripAfterUnlock(t *testing.T) {
	dir := t.TempDir()
	p := NewEncryptedFileProvider(dir)
	if err := p.Unlock("correct horse battery staple"); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if err := p.Set("id1", "super-secret-value"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := p.Get("id1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "super-secret-value" {
		t.Errorf("expected round-tripped value, got %q", got)
	}

	if err := p.Delete("id1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := p.Get("id1"); err != ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestEncryptedFileNeverStoresPlaintextOnDisk(t *testing.T) {
	dir := t.TempDir()
	p := NewEncryptedFileProvider(dir)
	if err := p.Unlock("passphrase"); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	const secret = "ghp_totallyRealTokenValue1234567890"
	if err := p.Set("id1", secret); err != nil {
		t.Fatalf("Set: %v", err)
	}

	data, err := os.ReadFile(p.path())
	if err != nil {
		t.Fatalf("reading vault file: %v", err)
	}
	if containsSubstring(string(data), secret) {
		t.Errorf("expected the raw secret value to never appear on disk, file contents: %s", data)
	}
}

func TestEncryptedFileWrongPassphraseFailsToDecrypt(t *testing.T) {
	dir := t.TempDir()
	p1 := NewEncryptedFileProvider(dir)
	if err := p1.Unlock("right-passphrase"); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if err := p1.Set("id1", "value"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	p2 := NewEncryptedFileProvider(dir)
	if err := p2.Unlock("wrong-passphrase"); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	if _, err := p2.Get("id1"); err == nil {
		t.Error("expected decrypting with the wrong passphrase to fail")
	}
}

func containsSubstring(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
