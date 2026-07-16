package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/auth"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/config"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/metrics"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/readiness"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

const oauthStateSessionKey = "state"

// Server serves metrics, OAuth, and readiness endpoints.
type Server struct {
	cfg     *config.Config
	auth    *auth.Manager
	metrics *metrics.Registry
	ready   *readiness.State
	store   *sessions.CookieStore
}

func New(cfg *config.Config, authMgr *auth.Manager, reg *metrics.Registry, ready *readiness.State) (*Server, error) {
	key := make([]byte, 24)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}

	return &Server{
		cfg:     cfg,
		auth:    authMgr,
		metrics: reg,
		ready:   ready,
		store:   sessions.NewCookieStore(key),
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(s.metrics.PrometheusRegistry(), promhttp.HandlerOpts{}))
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/oauth2callback", s.handleOAuthCallback)
	mux.HandleFunc("/readyz", s.handleReadyz)
	return mux
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf("0.0.0.0:%d", s.cfg.PromPort)
	log.Infof("Starting on port %d", s.cfg.PromPort)
	return http.ListenAndServe(addr, s.Handler())
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	ret := "<h1>prometheus-gmail-exporter</h1><br />"
	ret += "State: " + s.ready.Get() + "<br />"

	if s.auth.IsComplete() {
		ret += "Authenticated.<br />"
	} else {
		state, err := randomState()
		if err != nil {
			http.Error(w, "failed to generate oauth state", http.StatusInternalServerError)
			return
		}

		session, _ := s.store.Get(r, "oauth")
		session.Values[oauthStateSessionKey] = state
		if err := session.Save(r, w); err != nil {
			http.Error(w, "failed to save session", http.StatusInternalServerError)
			return
		}

		authURL, err := s.auth.AuthCodeURL(state)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		ret += fmt.Sprintf(`<a href="%s">Login</a>`, authURL)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(ret))
}

func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	session, _ := s.store.Get(r, "oauth")
	expectedState, _ := session.Values[oauthStateSessionKey].(string)
	if r.URL.Query().Get("state") != expectedState {
		http.Error(w, "Error: state mismatch", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := s.auth.Exchange(ctx, code); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Infof("Storing credentials to %s", s.cfg.CredentialsPath)
	_, _ = w.Write([]byte("Authentication successful. You can close this window."))
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if s.ready.IsReady() {
		_, _ = w.Write([]byte("OK"))
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte(s.ready.Get()))
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
