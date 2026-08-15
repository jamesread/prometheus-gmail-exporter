package readiness

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadinessStateMachine(t *testing.T) {
	ready := New()
	require.Equal(t, "STARTUP", ready.Get())
	require.False(t, ready.IsReady())

	ready.Set("SCRAPE_FAILED")
	require.Equal(t, "SCRAPE_FAILED", ready.Get())
	require.False(t, ready.IsReady())

	ready.Set("")
	require.True(t, ready.IsReady())
}
