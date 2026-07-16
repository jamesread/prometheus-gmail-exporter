package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

const gmailReadonlyScope = "https://www.googleapis.com/auth/gmail.readonly"

// CustomQuery mirrors the Python customQueries YAML entry.
type CustomQuery struct {
	Name  string `yaml:"name"`
	Query string `yaml:"query"`
}

// fileConfig is the YAML structure matching the Python exporter config file.
type fileConfig struct {
	Labels             []string      `yaml:"labels"`
	LabelsSenderCount  []string      `yaml:"labelsSenderCount"`
	ClientSecretFile   string        `yaml:"clientSecretFile"`
	CredentialsPath    string        `yaml:"credentialsPath"`
	UpdateDelaySeconds int           `yaml:"updateDelaySeconds"`
	OAuthHost          string        `yaml:"oauthHost"`
	OAuthRedirectURI   string        `yaml:"oauthRedirectUri"`
	OAuthBindAddr      string        `yaml:"oauthBindAddr"`
	OAuthBindPort      int           `yaml:"oauthBindPort"`
	PromPort           int           `yaml:"promPort"`
	Daemonize          bool          `yaml:"daemonize"`
	LogLevel           int           `yaml:"logLevel"`
	CustomQueries      []CustomQuery `yaml:"customQueries"`
}

// Config holds all runtime settings (CLI + YAML), matching the Python exporter.
type Config struct {
	Labels             []string
	LabelsSenderCount  []string
	ClientSecretFile   string
	CredentialsPath    string
	UpdateDelaySeconds int
	OAuthHost          string
	OAuthRedirectURI   string
	OAuthBindAddr      string
	OAuthBindPort      int
	PromPort           int
	Daemonize          bool
	LogLevel           int
	CustomQueries      []CustomQuery
}

func homedirFile(name string) string {
	dir := filepath.Join(os.Getenv("HOME"), ".prometheus-gmail-exporter")
	return filepath.Join(dir, name)
}

func defaultConfigPaths() []string {
	return []string{
		homedirFile("prometheus-gmail-exporter.cfg"),
		homedirFile("prometheus-gmail-exporter.yaml"),
		"/etc/prometheus-gmail-exporter.cfg",
		"/etc/prometheus-gmail-exporter.yaml",
	}
}

// Load parses config files and CLI flags in the same order as Python configargparse.
func Load(args []string) (*Config, error) {
	cfg := defaultConfig()

	var fileCfg fileConfig
	for _, path := range defaultConfigPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if err := yaml.Unmarshal(data, &fileCfg); err != nil {
			return nil, fmt.Errorf("load config %s: %w", path, err)
		}
		mergeFileConfig(cfg, &fileCfg)
	}

	fs := pflag.NewFlagSet("prometheus-gmail-exporter", pflag.ContinueOnError)
	fs.StringSlice("labels", nil, "Gmail label IDs to export")
	fs.StringSlice("labelsSenderCount", nil, "Labels for per-sender unread counts")
	fs.String("clientSecretFile", cfg.ClientSecretFile, "OAuth client secrets JSON")
	fs.String("credentialsPath", cfg.CredentialsPath, "Stored OAuth credentials")
	fs.Int("updateDelaySeconds", cfg.UpdateDelaySeconds, "Seconds between metric updates")
	fs.String("oauthHost", cfg.OAuthHost, "OAuth redirect host")
	fs.String("oauthRedirectUri", cfg.OAuthRedirectURI, "OAuth redirect URI override")
	fs.String("oauthBindAddr", cfg.OAuthBindAddr, "OAuth bind address (compatibility)")
	fs.Int("oauthBindPort", cfg.OAuthBindPort, "OAuth bind port (compatibility)")
	fs.Int("promPort", cfg.PromPort, "HTTP port for metrics and OAuth")
	fs.BoolP("daemonize", "d", cfg.Daemonize, "Run metric updates in a loop")
	fs.Int("logLevel", cfg.LogLevel, "Log level (Python logging numeric level)")
	fs.StringSlice("customQueries", nil, "Custom Gmail search queries (YAML fragments)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	mergeFlags(cfg, fs)
	cfg.expandPaths()
	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		ClientSecretFile:   homedirFile("client_secret.json"),
		CredentialsPath:    homedirFile("login_cookie.dat"),
		UpdateDelaySeconds: 300,
		OAuthHost:          "localhost",
		OAuthBindAddr:      "0.0.0.0",
		OAuthBindPort:      9090,
		PromPort:           8080,
		LogLevel:           20,
	}
}

func mergeFileConfig(cfg *Config, fileCfg *fileConfig) {
	if fileCfg.Labels != nil {
		cfg.Labels = fileCfg.Labels
	}
	if fileCfg.LabelsSenderCount != nil {
		cfg.LabelsSenderCount = fileCfg.LabelsSenderCount
	}
	if fileCfg.ClientSecretFile != "" {
		cfg.ClientSecretFile = fileCfg.ClientSecretFile
	}
	if fileCfg.CredentialsPath != "" {
		cfg.CredentialsPath = fileCfg.CredentialsPath
	}
	if fileCfg.UpdateDelaySeconds != 0 {
		cfg.UpdateDelaySeconds = fileCfg.UpdateDelaySeconds
	}
	if fileCfg.OAuthHost != "" {
		cfg.OAuthHost = fileCfg.OAuthHost
	}
	if fileCfg.OAuthRedirectURI != "" {
		cfg.OAuthRedirectURI = fileCfg.OAuthRedirectURI
	}
	if fileCfg.OAuthBindAddr != "" {
		cfg.OAuthBindAddr = fileCfg.OAuthBindAddr
	}
	if fileCfg.OAuthBindPort != 0 {
		cfg.OAuthBindPort = fileCfg.OAuthBindPort
	}
	if fileCfg.PromPort != 0 {
		cfg.PromPort = fileCfg.PromPort
	}
	cfg.Daemonize = fileCfg.Daemonize
	if fileCfg.LogLevel != 0 {
		cfg.LogLevel = fileCfg.LogLevel
	}
	if fileCfg.CustomQueries != nil {
		cfg.CustomQueries = fileCfg.CustomQueries
	}
}

func mergeFlags(cfg *Config, fs *pflag.FlagSet) {
	if fs.Changed("labels") {
		cfg.Labels = mustStringSlice(fs, "labels")
	}
	if fs.Changed("labelsSenderCount") {
		cfg.LabelsSenderCount = mustStringSlice(fs, "labelsSenderCount")
	}
	if fs.Changed("clientSecretFile") {
		cfg.ClientSecretFile = mustString(fs, "clientSecretFile")
	}
	if fs.Changed("credentialsPath") {
		cfg.CredentialsPath = mustString(fs, "credentialsPath")
	}
	if fs.Changed("updateDelaySeconds") {
		cfg.UpdateDelaySeconds = mustInt(fs, "updateDelaySeconds")
	}
	if fs.Changed("oauthHost") {
		cfg.OAuthHost = mustString(fs, "oauthHost")
	}
	if fs.Changed("oauthRedirectUri") {
		cfg.OAuthRedirectURI = mustString(fs, "oauthRedirectUri")
	}
	if fs.Changed("oauthBindAddr") {
		cfg.OAuthBindAddr = mustString(fs, "oauthBindAddr")
	}
	if fs.Changed("oauthBindPort") {
		cfg.OAuthBindPort = mustInt(fs, "oauthBindPort")
	}
	if fs.Changed("promPort") {
		cfg.PromPort = mustInt(fs, "promPort")
	}
	if fs.Changed("daemonize") {
		cfg.Daemonize = mustBool(fs, "daemonize")
	}
	if fs.Changed("logLevel") {
		cfg.LogLevel = mustInt(fs, "logLevel")
	}
}

func mustString(fs *pflag.FlagSet, name string) string {
	v, _ := fs.GetString(name)
	return v
}

func mustInt(fs *pflag.FlagSet, name string) int {
	v, _ := fs.GetInt(name)
	return v
}

func mustBool(fs *pflag.FlagSet, name string) bool {
	v, _ := fs.GetBool(name)
	return v
}

func mustStringSlice(fs *pflag.FlagSet, name string) []string {
	v, _ := fs.GetStringSlice(name)
	return v
}

func (c *Config) expandPaths() {
	c.ClientSecretFile = expandHome(c.ClientSecretFile)
	c.CredentialsPath = expandHome(c.CredentialsPath)
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(os.Getenv("HOME"), path[2:])
	}
	return path
}

// RedirectURI returns the OAuth callback URL, matching Python get_oauth_redirect_uri.
func (c *Config) RedirectURI() string {
	if c.OAuthRedirectURI != "" {
		return c.OAuthRedirectURI
	}
	return fmt.Sprintf("http://%s:%d/oauth2callback", c.OAuthHost, c.PromPort)
}

// Scopes returns the Gmail readonly scope list.
func (c *Config) Scopes() []string {
	return []string{gmailReadonlyScope}
}

func (c *Config) String() string {
	return fmt.Sprintf(
		"&{Labels:%v LabelsSenderCount:%v ClientSecretFile:%q CredentialsPath:%q UpdateDelaySeconds:%d OAuthHost:%q OAuthRedirectURI:%q OAuthBindAddr:%q OAuthBindPort:%d PromPort:%d Daemonize:%v LogLevel:%d CustomQueries:%v}",
		c.Labels, c.LabelsSenderCount, c.ClientSecretFile, c.CredentialsPath,
		c.UpdateDelaySeconds, c.OAuthHost, c.OAuthRedirectURI, c.OAuthBindAddr,
		c.OAuthBindPort, c.PromPort, c.Daemonize, c.LogLevel, c.CustomQueries,
	)
}

// EnsureConfigDir creates ~/.prometheus-gmail-exporter if needed.
func EnsureConfigDir() error {
	dir := filepath.Join(os.Getenv("HOME"), ".prometheus-gmail-exporter")
	return os.MkdirAll(dir, 0o700)
}
