package readiness

import (
	"sync/atomic"
)

// State tracks Kubernetes readiness; empty string means ready (matches Python).
type State struct {
	value atomic.Value
}

func New() *State {
	s := &State{}
	s.Set("STARTUP")
	return s
}

func (s *State) Set(message string) {
	s.value.Store(message)
}

func (s *State) Get() string {
	v := s.value.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

func (s *State) IsReady() bool {
	return s.Get() == ""
}
