package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

func TestMetricNamesMatchPython(t *testing.T) {
	reg := NewRegistry()

	reg.LabelTotal("INBOX", "INBOX").Set(10)
	reg.LabelUnread("INBOX", "INBOX").Set(2)
	reg.LabelSender("INBOX").WithLabelValues("alice@example.com").Set(1)
	reg.CustomQuery("fooquery").Set(201)

	require.Equal(t, 10.0, testutil.ToFloat64(reg.LabelTotal("INBOX", "INBOX")))
	require.Equal(t, MetricName("INBOX_total"), "gmail_INBOX_total")
	require.Equal(t, MetricName("INBOX_unread"), "gmail_INBOX_unread")
	require.Equal(t, MetricName("INBOX_sender"), "gmail_INBOX_sender")
	require.Equal(t, MetricName("fooquery"), "gmail_fooquery")
}
