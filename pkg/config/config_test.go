package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadYAMLFromREADMEExample(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfgPath := filepath.Join(dir, ".prometheus-gmail-exporter", "prometheus-gmail-exporter.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(cfgPath), 0o700))
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
labels:
  - Label_33
customQueries:
  - name: fooquery
    query: "important in:inbox"
`), 0o600))

	cfg, err := Load(nil)
	require.NoError(t, err)
	require.Equal(t, []string{"Label_33"}, cfg.Labels)
	require.Len(t, cfg.CustomQueries, 1)
	require.Equal(t, "fooquery", cfg.CustomQueries[0].Name)
	require.Equal(t, "important in:inbox", cfg.CustomQueries[0].Query)
}

func TestRedirectURIDefaultAndOverride(t *testing.T) {
	cfg := defaultConfig()
	cfg.OAuthHost = "localhost"
	cfg.PromPort = 8080
	require.Equal(t, "http://localhost:8080/oauth2callback", cfg.RedirectURI())

	cfg.OAuthRedirectURI = "http://example.com/callback"
	require.Equal(t, "http://example.com/callback", cfg.RedirectURI())
}

func TestCLIFlagsOverrideFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfgPath := filepath.Join(dir, ".prometheus-gmail-exporter", "prometheus-gmail-exporter.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(cfgPath), 0o700))
	require.NoError(t, os.WriteFile(cfgPath, []byte("labels:\n  - Label_33\n"), 0o600))

	cfg, err := Load([]string{"--labels", "INBOX"})
	require.NoError(t, err)
	require.Equal(t, []string{"INBOX"}, cfg.Labels)
}

func TestDefaultPaths(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg, err := Load(nil)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, ".prometheus-gmail-exporter", "client_secret.json"), cfg.ClientSecretFile)
	require.Equal(t, filepath.Join(dir, ".prometheus-gmail-exporter", "login_cookie.dat"), cfg.CredentialsPath)
	require.Equal(t, 300, cfg.UpdateDelaySeconds)
	require.Equal(t, 8080, cfg.PromPort)
}
