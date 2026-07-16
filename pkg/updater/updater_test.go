package updater

import (
	"context"
	"testing"

	"github.com/jamesread/prometheus-gmail-exporter/pkg/config"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/gmail"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/metrics"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/readiness"
	"github.com/prometheus/client_golang/prometheus/testutil"
	gmailapi "google.golang.org/api/gmail/v1"

	"github.com/stretchr/testify/require"
)

type mockUpdater struct {
	*Updater
	client gmail.API
}

func (m *mockUpdater) updateWithMock(ctx context.Context) error {
	threadSenderCache := make(map[string]string)

	logLabels, err := m.getLabels(ctx, m.client)
	if err != nil {
		return err
	}
	if len(logLabels) == 0 {
		return nil
	}

	for _, label := range logLabels {
		if err := m.updateLabel(ctx, m.client, label, threadSenderCache); err != nil {
			continue
		}
	}
	m.updateCustomQueries(ctx, m.client)
	m.ready.Set("")
	return nil
}

func TestUpdaterSetsLabelMetrics(t *testing.T) {
	cfg := &config.Config{Labels: []string{"INBOX"}}
	reg := metrics.NewRegistry()
	ready := readiness.New()

	up := &mockUpdater{
		Updater: New(cfg, nil, reg, ready),
		client: &gmail.MockAPI{
			Labels: []*gmailapi.Label{{
				Id:            "INBOX",
				Name:          "INBOX",
				ThreadsTotal:  44351,
				ThreadsUnread: 43,
			}},
		},
	}

	require.NoError(t, up.updateWithMock(context.Background()))
	require.True(t, ready.IsReady())
	require.Equal(t, 44351.0, testutil.ToFloat64(reg.LabelTotal("INBOX", "INBOX")))
	require.Equal(t, 43.0, testutil.ToFloat64(reg.LabelUnread("INBOX", "INBOX")))
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

	up := &mockUpdater{
		Updater: New(cfg, nil, reg, ready),
		client: &gmail.MockAPI{
			Labels: []*gmailapi.Label{{
				Id: "INBOX", Name: "INBOX", ThreadsTotal: 1, ThreadsUnread: 0,
			}},
			MessageEstimates: map[string]int64{
				"important in:inbox": 201,
			},
		},
	}

	require.NoError(t, up.updateWithMock(context.Background()))
	require.Equal(t, 201.0, testutil.ToFloat64(reg.CustomQuery("fooquery")))
}

func TestGoldenMetricsOutput(t *testing.T) {
	cfg := &config.Config{Labels: []string{"INBOX"}}
	reg := metrics.NewRegistry()
	ready := readiness.New()

	up := &mockUpdater{
		Updater: New(cfg, nil, reg, ready),
		client: &gmail.MockAPI{
			Labels: []*gmailapi.Label{{
				Id: "INBOX", Name: "INBOX", ThreadsTotal: 44351, ThreadsUnread: 43,
			}},
		},
	}
	require.NoError(t, up.updateWithMock(context.Background()))

	require.Equal(t, 44351.0, testutil.ToFloat64(reg.LabelTotal("INBOX", "INBOX")))
	require.Equal(t, 43.0, testutil.ToFloat64(reg.LabelUnread("INBOX", "INBOX")))

	mfs, err := reg.PrometheusRegistry().Gather()
	require.NoError(t, err)
	names := make([]string, 0, len(mfs))
	for _, mf := range mfs {
		names = append(names, mf.GetName())
	}
	require.Contains(t, names, metrics.MetricName("INBOX_total"))
	require.Contains(t, names, metrics.MetricName("INBOX_unread"))
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
