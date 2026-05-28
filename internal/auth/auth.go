package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

const (
	clientID    = ""           // set via SPOTUI_CLIENT_ID env var
	redirectURI = "http://127.0.0.1:8090/callback"
	authURL     = "https://accounts.spotify.com/authorize"
	tokenURL    = "https://accounts.spotify.com/api/token"

	scopes = "user-read-playback-state user-modify-playback-state " +
		"user-read-currently-playing playlist-read-private " +
		"playlist-read-collaborative user-library-read user-read-private"
)

type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func (t *Token) Valid() bool {
	return t != nil && t.AccessToken != "" && time.Now().Before(t.ExpiresAt.Add(-30*time.Second))
}

type pkceState struct {
	verifier  string
	challenge string
	state     string
}

func newPKCE() (pkceState, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return pkceState{}, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(raw)

	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return pkceState{}, err
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	return pkceState{verifier: verifier, challenge: challenge, state: state}, nil
}

// Login performs the OAuth2 PKCE flow, opening the browser for the user.
// Returns a valid Token on success.
func Login(clientID string) (*Token, error) {
	pkce, err := newPKCE()
	if err != nil {
		return nil, fmt.Errorf("pkce: %w", err)
	}

	params := url.Values{
		"client_id":             {clientID},
		"response_type":         {"code"},
		"redirect_uri":          {redirectURI},
		"scope":                 {scopes},
		"code_challenge_method": {"S256"},
		"code_challenge":        {pkce.challenge},
		"state":                 {pkce.state},
		"show_dialog":           {"true"}, // force consent screen so Spotify re-grants all scopes
	}
	authorizationURL := authURL + "?" + params.Encode()

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	srv := &http.Server{Addr: ":8090"}
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != pkce.state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			errCh <- fmt.Errorf("oauth state mismatch")
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- fmt.Errorf("no code in callback")
			return
		}
		fmt.Fprintln(w, "<html><body><h2>SpoTUI — authorized! You can close this tab.</h2></body></html>")
		codeCh <- code
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	fmt.Printf("\nOpening browser for Spotify login...\nIf it doesn't open, visit:\n\n  %s\n\n", authorizationURL)
	openBrowser(authorizationURL)

	var code string
	select {
	case code = <-codeCh:
	case err = <-errCh:
		return nil, err
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("auth timeout")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go srv.Shutdown(ctx) //nolint

	token, err := exchangeCode(clientID, code, pkce.verifier)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}

	return token, nil
}

func exchangeCode(clientID, code, verifier string) (*Token, error) {
	resp, err := http.PostForm(tokenURL, map[string][]string{
		"client_id":     {clientID},
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	if raw.Error != "" {
		return nil, fmt.Errorf("spotify: %s", raw.Error)
	}

	return &Token{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second),
	}, nil
}

// Refresh gets a new access token using the stored refresh token.
func Refresh(clientID string, t *Token) (*Token, error) {
	resp, err := http.PostForm(tokenURL, map[string][]string{
		"client_id":     {clientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {t.RefreshToken},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	if raw.Error != "" {
		return nil, fmt.Errorf("spotify refresh: %s", raw.Error)
	}

	newToken := &Token{
		AccessToken:  raw.AccessToken,
		ExpiresAt:    time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second),
		RefreshToken: t.RefreshToken,
	}
	if raw.RefreshToken != "" {
		newToken.RefreshToken = raw.RefreshToken
	}
	return newToken, nil
}

func TokenPath() string {
	cfg, _ := os.UserConfigDir()
	return filepath.Join(cfg, "spotui", "token.json")
}

func SaveToken(t *Token) error {
	path := TokenPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(t)
}

func LoadToken() (*Token, error) {
	f, err := os.Open(TokenPath())
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var t Token
	if err := json.NewDecoder(f).Decode(&t); err != nil {
		return nil, err
	}
	return &t, nil
}
