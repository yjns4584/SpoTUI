package spotify

import (
	"fmt"
	"sync"

	"github.com/yesid/spotui/internal/auth"
)

// TokenManager wraps auth.Token and auto-refreshes when needed.
type TokenManager struct {
	mu       sync.Mutex
	token    *auth.Token
	clientID string
}

func NewTokenManager(clientID string, token *auth.Token) *TokenManager {
	return &TokenManager{clientID: clientID, token: token}
}

func (m *TokenManager) AccessToken() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.token.Valid() {
		return m.token.AccessToken, nil
	}

	refreshed, err := auth.Refresh(m.clientID, m.token)
	if err != nil {
		return "", fmt.Errorf("refresh token: %w", err)
	}

	m.token = refreshed
	_ = auth.SaveToken(refreshed)
	return m.token.AccessToken, nil
}
