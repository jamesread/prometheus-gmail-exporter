package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/jamesread/prometheus-gmail-exporter/pkg/config"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const userAgent = "prometheus-gmail-exporter"

// storedCredentials matches the JSON written by Python google-auth Credentials.to_json().
type storedCredentials struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	TokenURI     string `json:"token_uri"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Scopes       []string `json:"scopes"`
	Expiry       string `json:"expiry"`
}

// Manager handles OAuth credentials and Gmail client construction.
type Manager struct {
	cfg      *config.Config
	mu       sync.RWMutex
	complete bool
	oauthCfg *oauth2.Config
	token    *oauth2.Token
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{cfg: cfg}
}

func (m *Manager) IsComplete() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.complete
}

func (m *Manager) SetComplete(complete bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.complete = complete
}

// TryMarkComplete loads stored credentials on startup, matching Python try_mark_auth_complete.
func (m *Manager) TryMarkComplete() {
	if _, err := os.Stat(m.cfg.CredentialsPath); os.IsNotExist(err) {
		log.Infof("No credentials file at %s; OAuth login required", m.cfg.CredentialsPath)
		return
	}

	creds, err := m.loadCredentials()
	if err != nil {
		log.Warningf("Credentials at %s are missing or invalid; OAuth login required", m.cfg.CredentialsPath)
		return
	}

	if creds != nil && creds.Valid() {
		m.mu.Lock()
		m.token = creds
		m.complete = true
		m.mu.Unlock()
		log.Infof("Loaded valid credentials from %s", m.cfg.CredentialsPath)
		return
	}

	log.Warningf("Credentials at %s are missing or invalid; OAuth login required", m.cfg.CredentialsPath)
}

func (m *Manager) OAuthConfig() (*oauth2.Config, error) {
	if m.oauthCfg != nil {
		return m.oauthCfg, nil
	}

	for {
		if _, err := os.Stat(m.cfg.ClientSecretFile); err == nil {
			break
		}
		log.Fatalf("Client secrets file does not exist: %s . You probably need to download this from the Google API console.", m.cfg.ClientSecretFile)
		time.Sleep(10 * time.Second)
	}

	data, err := os.ReadFile(m.cfg.ClientSecretFile)
	if err != nil {
		return nil, err
	}

	oauthCfg, err := google.ConfigFromJSON(data, m.cfg.Scopes()...)
	if err != nil {
		return nil, err
	}
	oauthCfg.RedirectURL = m.cfg.RedirectURI()
	m.oauthCfg = oauthCfg
	return oauthCfg, nil
}

func (m *Manager) AuthCodeURL(state string) (string, error) {
	oauthCfg, err := m.OAuthConfig()
	if err != nil {
		return "", err
	}
	return oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline), nil
}

func (m *Manager) Exchange(ctx context.Context, code string) error {
	oauthCfg, err := m.OAuthConfig()
	if err != nil {
		return err
	}

	token, err := oauthCfg.Exchange(ctx, code)
	if err != nil {
		return err
	}

	if err := m.saveToken(token, oauthCfg); err != nil {
		return err
	}

	m.mu.Lock()
	m.token = token
	m.complete = true
	m.mu.Unlock()

	return nil
}

func (m *Manager) loadCredentials() (*oauth2.Token, error) {
	data, err := os.ReadFile(m.cfg.CredentialsPath)
	if err != nil {
		return nil, err
	}

	var stored storedCredentials
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, err
	}

	token := &oauth2.Token{
		AccessToken:  stored.Token,
		RefreshToken: stored.RefreshToken,
		TokenType:    "Bearer",
	}

	if stored.Expiry != "" {
		if expiry, err := time.Parse(time.RFC3339, stored.Expiry); err == nil {
			token.Expiry = expiry
		} else if expiry, err := time.Parse(time.RFC3339Nano, stored.Expiry); err == nil {
			token.Expiry = expiry
		}
	}

	if !token.Valid() && token.RefreshToken != "" {
		log.Info("Refreshing expired credentials")
		oauthCfg, err := m.OAuthConfig()
		if err != nil {
			return nil, err
		}
		ctx := context.Background()
		src := oauthCfg.TokenSource(ctx, token)
		refreshed, err := src.Token()
		if err != nil {
			return nil, err
		}
		if err := m.saveToken(refreshed, oauthCfg); err != nil {
			return nil, err
		}
		token = refreshed
	} else if !token.Valid() {
		return nil, fmt.Errorf("token invalid and no refresh token")
	}

	m.mu.Lock()
	m.token = token
	m.mu.Unlock()

	return token, nil
}

func (m *Manager) saveToken(token *oauth2.Token, oauthCfg *oauth2.Config) error {
	log.Infof("Storing credentials to %s", m.cfg.CredentialsPath)

	stored := storedCredentials{
		Token:        token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenURI:     oauthCfg.Endpoint.TokenURL,
		ClientID:     oauthCfg.ClientID,
		ClientSecret: oauthCfg.ClientSecret,
		Scopes:       m.cfg.Scopes(),
	}
	if !token.Expiry.IsZero() {
		stored.Expiry = token.Expiry.Format(time.RFC3339)
	}

	data, err := json.Marshal(stored)
	if err != nil {
		return err
	}

	return os.WriteFile(m.cfg.CredentialsPath, data, 0o600)
}

// WaitForAuth blocks until OAuth completes, matching Python get_gmail_client.
func (m *Manager) WaitForAuth() {
	for !m.IsComplete() {
		log.Infof("Waiting for credentials, sleeping for %d seconds", m.cfg.UpdateDelaySeconds)
		time.Sleep(time.Duration(m.cfg.UpdateDelaySeconds) * time.Second)
	}
}

// GmailService returns an authenticated Gmail API client.
func (m *Manager) GmailService(ctx context.Context) (*gmail.Service, error) {
	m.WaitForAuth()

	token, err := m.loadCredentials()
	if err != nil {
		return nil, err
	}

	oauthCfg, err := m.OAuthConfig()
	if err != nil {
		return nil, err
	}

	client := oauth2.NewClient(ctx, oauthCfg.TokenSource(ctx, token))
	client.Transport = &userAgentTransport{base: client.Transport, agent: userAgent}

	return gmail.NewService(ctx, option.WithHTTPClient(client))
}

type userAgentTransport struct {
	base  http.RoundTripper
	agent string
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Header.Set("User-Agent", t.agent)
	return t.base.RoundTrip(req2)
}

// TokenFromStored parses a Python-written credentials file (used in tests).
func TokenFromStored(data []byte) (*oauth2.Token, error) {
	var stored storedCredentials
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, err
	}
	token := &oauth2.Token{
		AccessToken:  stored.Token,
		RefreshToken: stored.RefreshToken,
		TokenType:    "Bearer",
	}
	if stored.Expiry != "" {
		if expiry, err := time.Parse(time.RFC3339, stored.Expiry); err == nil {
			token.Expiry = expiry
		}
	}
	return token, nil
}
