package providers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/argon2"
)

const (
	encryptedFileName = "vault_secrets.enc.json"
	saltLen           = 16
	keyLen            = 32 // AES-256
)

// ErrConsentRequired is returned by any operation on EncryptedFileProvider
// before Unlock has been called. This provider must never activate itself
// implicitly — a caller (backed by an explicit user prompt) must supply a
// passphrase first.
var ErrConsentRequired = errors.New("providers: encrypted-file fallback requires explicit user consent (call Unlock first)")

type encryptedFileDoc struct {
	Salt    string            `json:"salt"`
	Secrets map[string]string `json:"secrets"` // id -> base64(nonce || ciphertext)
}

// EncryptedFileProvider is a fallback CredentialProvider used only when no
// OS credential service is available. It stores AES-256-GCM-encrypted
// values on disk, with the key derived (Argon2id) from a passphrase the
// caller must explicitly supply via Unlock — there is no silent, implicit,
// or plaintext fallback path.
type EncryptedFileProvider struct {
	mu  sync.Mutex
	dir string
	key []byte
}

// NewEncryptedFileProvider constructs the fallback provider rooted at dir.
// It remains locked (unusable) until Unlock is called.
func NewEncryptedFileProvider(dir string) *EncryptedFileProvider {
	return &EncryptedFileProvider{dir: dir}
}

func (p *EncryptedFileProvider) Kind() string { return "encrypted_file" }

// Unlock derives the encryption key from a user-supplied passphrase. This
// call is the explicit-consent gate: nothing in this package invokes it
// automatically.
func (p *EncryptedFileProvider) Unlock(passphrase string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	doc, err := p.load()
	if err != nil {
		return err
	}
	salt, err := p.saltBytes(&doc)
	if err != nil {
		return err
	}
	p.key = argon2.IDKey([]byte(passphrase), salt, 3, 64*1024, 4, keyLen)
	return p.save(doc)
}

// saltBytes returns the doc's existing salt, generating and persisting a new
// one into doc.Salt on first use.
func (p *EncryptedFileProvider) saltBytes(doc *encryptedFileDoc) ([]byte, error) {
	if doc.Salt != "" {
		return hex.DecodeString(doc.Salt)
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	doc.Salt = hex.EncodeToString(salt)
	return salt, nil
}

func (p *EncryptedFileProvider) path() string {
	return filepath.Join(p.dir, encryptedFileName)
}

func (p *EncryptedFileProvider) load() (encryptedFileDoc, error) {
	var doc encryptedFileDoc
	data, err := os.ReadFile(p.path())
	if err != nil {
		if os.IsNotExist(err) {
			doc.Secrets = map[string]string{}
			return doc, nil
		}
		return doc, err
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return doc, err
	}
	if doc.Secrets == nil {
		doc.Secrets = map[string]string{}
	}
	return doc, nil
}

func (p *EncryptedFileProvider) save(doc encryptedFileDoc) error {
	if err := os.MkdirAll(p.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	tmp := p.path() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p.path())
}

func (p *EncryptedFileProvider) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(p.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (p *EncryptedFileProvider) decrypt(encoded string) (string, error) {
	block, err := aes.NewCipher(p.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("providers: ciphertext too short")
	}
	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (p *EncryptedFileProvider) Get(id string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.key == nil {
		return "", ErrConsentRequired
	}
	doc, err := p.load()
	if err != nil {
		return "", err
	}
	enc, ok := doc.Secrets[id]
	if !ok {
		return "", ErrNotFound
	}
	return p.decrypt(enc)
}

func (p *EncryptedFileProvider) Set(id, value string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.key == nil {
		return ErrConsentRequired
	}
	doc, err := p.load()
	if err != nil {
		return err
	}
	enc, err := p.encrypt(value)
	if err != nil {
		return err
	}
	doc.Secrets[id] = enc
	return p.save(doc)
}

func (p *EncryptedFileProvider) Delete(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.key == nil {
		return ErrConsentRequired
	}
	doc, err := p.load()
	if err != nil {
		return err
	}
	delete(doc.Secrets, id)
	return p.save(doc)
}
