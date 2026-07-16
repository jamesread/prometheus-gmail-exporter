package readiness

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadinessStateMachine(t *testing.T) {
	ready := New()
	require.Equal(t, "STARTUP", ready.Get())
	require.False(t, ready.IsReady())

	ready.Set("MAIN")
	require.Equal(t, "MAIN", ready.Get())

	ready.Set("GET_CREDENTIALS")
	ready.Set("GOT_CREDENTIALS")
	require.Equal(t, "GOT_CREDENTIALS", ready.Get())

	ready.Set("")
	require.True(t, ready.IsReady())
}
