package updater

import (
	"context"
	"errors"
	"testing"

	"github.com/jamesread/prometheus-gmail-exporter/pkg/config"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/gmail"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/metrics"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/readiness"
	"github.com/prometheus/client_golang/prometheus/testutil"
	gmailapi "google.golang.org/api/gmail/v1"

	"github.com/stretchr/testify/require"
)

func TestUpdaterSetsLabelMetrics(t *testing.T) {
	cfg := &config.Config{Labels: []string{"INBOX"}}
	reg := metrics.NewRegistry()
	ready := readiness.New()

	up := New(cfg, nil, reg, ready)
	err := up.updateWithClient(context.Background(), &gmail.MockAPI{
		Labels: []*gmailapi.Label{{
			Id:            "INBOX",
			Name:          "INBOX",
			ThreadsTotal:  44351,
			ThreadsUnread: 43,
		}},
	})

	require.NoError(t, err)
	require.True(t, ready.IsReady())
	require.Equal(t, 44351.0, testutil.ToFloat64(reg.LabelTotal("INBOX", "INBOX")))
	require.Equal(t, 43.0, testutil.ToFloat64(reg.LabelUnread("INBOX", "INBOX")))
	require.Equal(t, 1.0, testutil.ToFloat64(reg.ScrapeSuccess()))
}

func TestUpdaterCustomQueryMetrics(t *testing.T) {
	cfg := &config.Config{
		Labels: []string{"INBOX"},
		CustomQueries: []config.CustomQuery{{
			Name:  "fooquery",
			Query: "important in:inbox",
		}},
	}
	reg := metrics.NewRegistry()
	ready := readiness.New()

	up := New(cfg, nil, reg, ready)
	err := up.updateWithClient(context.Background(), &gmail.MockAPI{
		Labels: []*gmailapi.Label{{
			Id: "INBOX", Name: "INBOX", ThreadsTotal: 1, ThreadsUnread: 0,
		}},
		MessageEstimates: map[string]int64{
			"important in:inbox": 201,
		},
	})

	require.NoError(t, err)
	require.Equal(t, 201.0, testutil.ToFloat64(reg.CustomQuery("fooquery")))
	require.Equal(t, 1.0, testutil.ToFloat64(reg.ScrapeSuccess()))
}

func TestGoldenMetricsOutput(t *testing.T) {
	cfg := &config.Config{Labels: []string{"INBOX"}}
	reg := metrics.NewRegistry()
	ready := readiness.New()

	up := New(cfg, nil, reg, ready)
	require.NoError(t, up.updateWithClient(context.Background(), &gmail.MockAPI{
		Labels: []*gmailapi.Label{{
			Id: "INBOX", Name: "INBOX", ThreadsTotal: 44351, ThreadsUnread: 43,
		}},
	}))

	require.Equal(t, 44351.0, testutil.ToFloat64(reg.LabelTotal("INBOX", "INBOX")))
	require.Equal(t, 43.0, testutil.ToFloat64(reg.LabelUnread("INBOX", "INBOX")))

	mfs, err := reg.PrometheusRegistry().Gather()
	require.NoError(t, err)
	names := make([]string, 0, len(mfs))
	for _, mf := range mfs {
		names = append(names, mf.GetName())
	}
	require.Contains(t, names, "gmail_label_total")
	require.Contains(t, names, "gmail_label_unread")
	require.Contains(t, names, "gmail_scrape_success")
}

func TestPartialLabelFailureDeletesSeries(t *testing.T) {
	cfg := &config.Config{Labels: []string{"INBOX", "SPAM"}}
	reg := metrics.NewRegistry()
	ready := readiness.New()
	up := New(cfg, nil, reg, ready)

	client := &gmail.MockAPI{
		Labels: []*gmailapi.Label{
			{Id: "INBOX", Name: "INBOX", ThreadsTotal: 10, ThreadsUnread: 2},
			{Id: "SPAM", Name: "SPAM", ThreadsTotal: 5, ThreadsUnread: 1},
		},
	}
	require.NoError(t, up.updateWithClient(context.Background(), client))
	require.Equal(t, 10.0, testutil.ToFloat64(reg.LabelTotal("INBOX", "INBOX")))
	require.Equal(t, 5.0, testutil.ToFloat64(reg.LabelTotal("SPAM", "SPAM")))

	client.GetLabelErrs = map[string]error{"SPAM": errors.New("api down")}
	err := up.updateWithClient(context.Background(), client)
	require.Error(t, err)
	require.Equal(t, "SCRAPE_FAILED", ready.Get())
	require.Equal(t, 0.0, testutil.ToFloat64(reg.ScrapeSuccess()))
	require.Equal(t, 10.0, testutil.ToFloat64(reg.LabelTotal("INBOX", "INBOX")))

	mfs, err := reg.PrometheusRegistry().Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() != "gmail_label_total" && mf.GetName() != "gmail_label_unread" {
			continue
		}
		for _, metric := range mf.GetMetric() {
			for _, lp := range metric.GetLabel() {
				if lp.GetName() == "id" {
					require.NotEqual(t, "SPAM", lp.GetValue())
				}
			}
		}
	}
}

func TestFullFailureClearsAllSeries(t *testing.T) {
	cfg := &config.Config{Labels: []string{}}
	reg := metrics.NewRegistry()
	ready := readiness.New()
	up := New(cfg, nil, reg, ready)

	reg.SetLabelTotal("INBOX", "INBOX", 99)
	err := up.updateWithClient(context.Background(), &gmail.MockAPI{
		ListLabelsErr: errors.New("list failed"),
	})
	require.Error(t, err)
	require.Equal(t, 0.0, testutil.ToFloat64(reg.ScrapeSuccess()))

	mfs, gatherErr := reg.PrometheusRegistry().Gather()
	require.NoError(t, gatherErr)
	for _, mf := range mfs {
		if mf.GetName() == "gmail_label_total" {
			require.Empty(t, mf.GetMetric())
		}
	}
}

func TestFirstMessageSender(t *testing.T) {
	thread := &gmailapi.Thread{
		Messages: []*gmailapi.Message{{
			Payload: &gmailapi.MessagePart{
				Headers: []*gmailapi.MessagePartHeader{{Name: "From", Value: "a@b.com"}},
			},
		}},
	}
	require.Equal(t, "a@b.com", gmail.FirstMessageSender(thread))
	require.Equal(t, "unknown-thread-no-messages", gmail.FirstMessageSender(nil))
}
