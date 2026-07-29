package github

import (
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

const keyringService = "reqit-github"

type AuthService struct {
	account string
}

func NewAuthService(account string) *AuthService {
	account = strings.TrimSpace(account)
	if account == "" {
		account = "default"
	}
	return &AuthService{account: account}
}

func (a *AuthService) SaveToken(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token is required")
	}
	return keyring.Set(keyringService, a.account, token)
}

func (a *AuthService) LoadToken() (string, error) {
	return keyring.Get(keyringService, a.account)
}

func (a *AuthService) DeleteToken() error {
	return keyring.Delete(keyringService, a.account)
}
