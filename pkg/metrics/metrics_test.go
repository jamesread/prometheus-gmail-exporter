package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestLabeledMetricNames(t *testing.T) {
	reg := NewRegistry()

	reg.SetLabelTotal("INBOX", "INBOX", 10)
	reg.SetLabelUnread("INBOX", "INBOX", 2)
	reg.SetLabelSender("INBOX", "INBOX", "alice@example.com", 1)
	reg.SetCustomQuery("fooquery", 201)

	require.Equal(t, 10.0, testutil.ToFloat64(reg.LabelTotal("INBOX", "INBOX")))
	require.Equal(t, 2.0, testutil.ToFloat64(reg.LabelUnread("INBOX", "INBOX")))
	require.Equal(t, 1.0, testutil.ToFloat64(reg.LabelSender("INBOX", "INBOX").WithLabelValues("alice@example.com")))
	require.Equal(t, 201.0, testutil.ToFloat64(reg.CustomQuery("fooquery")))

	mfs, err := reg.PrometheusRegistry().Gather()
	require.NoError(t, err)
	names := make(map[string]bool, len(mfs))
	for _, mf := range mfs {
		names[mf.GetName()] = true
	}
	require.True(t, names["gmail_label_total"])
	require.True(t, names["gmail_label_unread"])
	require.True(t, names["gmail_label_sender"])
	require.True(t, names["gmail_custom_query"])
	require.False(t, names["gmail_INBOX_total"])
	require.False(t, names["gmail_INBOX_unread"])
}

func TestDeleteLabelRemovesSeries(t *testing.T) {
	reg := NewRegistry()
	reg.SetLabelTotal("INBOX", "INBOX", 10)
	reg.SetLabelUnread("INBOX", "INBOX", 2)
	reg.SetLabelSender("INBOX", "INBOX", "alice@example.com", 1)

	reg.DeleteLabel("INBOX", "INBOX")

	mfs, err := reg.PrometheusRegistry().Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		switch mf.GetName() {
		case "gmail_label_total", "gmail_label_unread", "gmail_label_sender":
			require.Empty(t, mf.GetMetric())
		}
	}
}
