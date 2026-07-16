package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jamesread/prometheus-gmail-exporter/pkg/config"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/readiness"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

const pythonCredentialFixture = `{
  "token": "ya29.example",
  "refresh_token": "1//example",
  "token_uri": "https://oauth2.googleapis.com/token",
  "client_id": "123.apps.googleusercontent.com",
  "client_secret": "secret",
  "scopes": ["https://www.googleapis.com/auth/gmail.readonly"],
  "expiry": "2030-01-01T00:00:00Z"
}`

func TestTokenFromStoredPythonFixture(t *testing.T) {
	token, err := TokenFromStored([]byte(pythonCredentialFixture))
	require.NoError(t, err)
	require.Equal(t, "ya29.example", token.AccessToken)
	require.Equal(t, "1//example", token.RefreshToken)
	require.True(t, token.Valid())
}

func TestSaveTokenMatchesPythonShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "login_cookie.dat")

	cfg := &config.Config{CredentialsPath: path}
	mgr := NewManager(cfg, readiness.New())

	token := &oauth2.Token{
		AccessToken:  "access",
		RefreshToken: "refresh",
		Expiry:       time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	oauthCfg := &oauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://oauth2.googleapis.com/token",
		},
	}

	require.NoError(t, mgr.saveToken(token, oauthCfg))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var stored storedCredentials
	require.NoError(t, json.Unmarshal(data, &stored))
	require.Equal(t, "access", stored.Token)
	require.Equal(t, "refresh", stored.RefreshToken)
	require.Equal(t, "client-id", stored.ClientID)
	require.Equal(t, "client-secret", stored.ClientSecret)
	require.Equal(t, "https://oauth2.googleapis.com/token", stored.TokenURI)
	require.Contains(t, stored.Scopes, "https://www.googleapis.com/auth/gmail.readonly")
}

func TestTryMarkCompleteWithPythonFixture(t *testing.T) {
	dir := t.TempDir()
	credPath := filepath.Join(dir, "login_cookie.dat")
	require.NoError(t, os.WriteFile(credPath, []byte(pythonCredentialFixture), 0o600))

	secretPath := filepath.Join(dir, "client_secret.json")
	require.NoError(t, os.WriteFile(secretPath, []byte(`{
  "installed": {
    "client_id": "123.apps.googleusercontent.com",
    "client_secret": "secret",
    "redirect_uris": ["http://localhost"]
  }
}`), 0o600))

	cfg := &config.Config{
		CredentialsPath:  credPath,
		ClientSecretFile: secretPath,
	}
	ready := readiness.New()
	mgr := NewManager(cfg, ready)
	mgr.TryMarkComplete()

	require.True(t, mgr.IsComplete())
	require.Equal(t, "GOT_CREDENTIALS", ready.Get())
}
