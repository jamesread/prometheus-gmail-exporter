package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jamesread/prometheus-gmail-exporter/pkg/auth"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/config"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/metrics"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/readiness"
	"github.com/stretchr/testify/require"
)

func TestReadyzReadyAndNotReady(t *testing.T) {
	cfg := &config.Config{PromPort: 8080}
	ready := readiness.New()
	authMgr := auth.NewManager(cfg, ready)
	srv, err := New(cfg, authMgr, metrics.NewRegistry(), ready)
	require.NoError(t, err)

	ready.Set("MAIN")
	rec := httptest.NewRecorder()
	srv.handleReadyz(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Equal(t, "MAIN", rec.Body.String())

	ready.Set("")
	rec = httptest.NewRecorder()
	srv.handleReadyz(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "OK", rec.Body.String())
}

func TestIndexShowsLoginWhenUnauthenticated(t *testing.T) {
	dir := t.TempDir()
	secretPath := dir + "/client_secret.json"
	require.NoError(t, os.WriteFile(secretPath, []byte(`{
  "installed": {
    "client_id": "id",
    "client_secret": "secret",
    "redirect_uris": ["http://localhost"]
  }
}`), 0o600))

	cfg := &config.Config{
		PromPort:         8080,
		ClientSecretFile: secretPath,
		OAuthHost:        "localhost",
	}
	ready := readiness.New()
	authMgr := auth.NewManager(cfg, ready)
	srv, err := New(cfg, authMgr, metrics.NewRegistry(), ready)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	srv.handleIndex(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "Login</a>")
}
