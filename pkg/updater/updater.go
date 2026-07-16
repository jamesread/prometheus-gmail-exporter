package updater

import (
	"context"
	"sync"

	"github.com/jamesread/prometheus-gmail-exporter/pkg/auth"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/config"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/gmail"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/metrics"
	"github.com/jamesread/prometheus-gmail-exporter/pkg/readiness"
	log "github.com/sirupsen/logrus"
)

// Updater refreshes Gmail metrics on a schedule.
type Updater struct {
	cfg          *config.Config
	auth         *auth.Manager
	metrics      *metrics.Registry
	ready        *readiness.State
	senderLabels map[string]struct{}
	labelCache   []gmail.LabelRef
	labelOnce    sync.Once
}

func New(cfg *config.Config, authMgr *auth.Manager, reg *metrics.Registry, ready *readiness.State) *Updater {
	senderLabels := make(map[string]struct{}, len(cfg.LabelsSenderCount))
	for _, id := range cfg.LabelsSenderCount {
		senderLabels[id] = struct{}{}
	}
	return &Updater{
		cfg:          cfg,
		auth:         authMgr,
		metrics:      reg,
		ready:        ready,
		senderLabels: senderLabels,
	}
}

// UpdateOnce performs a single metrics refresh cycle.
func (u *Updater) UpdateOnce(ctx context.Context) error {
	threadSenderCache := make(map[string]string)

	svc, err := u.auth.GmailService(ctx)
	if err != nil {
		return err
	}
	client := gmail.NewClient(svc)

	log.Info("Got gmail client successfully")

	log.Info("Updating gmail metrics - started")

	labels, err := u.getLabels(ctx, client)
	if err != nil {
		return err
	}
	if len(labels) == 0 {
		log.Warning("Skipping metric update: no labels configured or found")
		return nil
	}

	for _, label := range labels {
		if err := u.updateLabel(ctx, client, label, threadSenderCache); err != nil {
			log.Errorf("Error: %v", err)
		}
	}

	log.Info("Updating gmail metrics - complete")

	u.updateCustomQueries(ctx, client)

	u.ready.Set("")
	return nil
}

func (u *Updater) getLabels(ctx context.Context, client gmail.API) ([]gmail.LabelRef, error) {
	var err error
	u.labelOnce.Do(func() {
		log.Info("Getting metadata about labels")

		if len(u.cfg.Labels) == 0 {
			log.Warning("No labels specified, assuming all labels. If you have a lot of labels in your inbox you could hit API limits quickly.")
			all, listErr := client.ListLabels(ctx)
			if listErr != nil {
				err = listErr
				return
			}
			for _, label := range all {
				u.labelCache = append(u.labelCache, gmail.LabelRef{ID: label.Id})
			}
		} else {
			log.Infof("Using labels: %v", u.cfg.Labels)
			for _, id := range u.cfg.Labels {
				u.labelCache = append(u.labelCache, gmail.LabelRef{ID: id})
			}
		}

		if len(u.labelCache) == 0 {
			log.Warning("No labels found.")
		}
	})

	if err != nil {
		return nil, err
	}
	return u.labelCache, nil
}

func (u *Updater) updateLabel(ctx context.Context, client gmail.API, label gmail.LabelRef, senderCache map[string]string) error {
	labelInfo, err := client.GetLabel(ctx, label.ID)
	if err != nil {
		return err
	}

	u.metrics.SetLabelTotal(labelInfo.Id, labelInfo.Name, float64(labelInfo.ThreadsTotal))
	u.metrics.SetLabelUnread(labelInfo.Id, labelInfo.Name, float64(labelInfo.ThreadsUnread))

	if _, ok := u.senderLabels[label.ID]; ok {
		u.updateSenderGauges(ctx, client, labelInfo.Id, labelInfo.Name, senderCache)
	}
	return nil
}

func (u *Updater) updateSenderGauges(ctx context.Context, client gmail.API, labelID, labelName string, senderCache map[string]string) {
	senderCounts := make(map[string]int)

	threads, err := u.getAllUnreadThreads(ctx, client, labelID)
	if err != nil {
		log.Errorf("Error: %v", err)
		return
	}

	for _, threadID := range threads {
		sender, ok := senderCache[threadID]
		if !ok {
			full, err := client.GetThreadMetadata(ctx, threadID)
			if err != nil {
				log.Errorf("Error: %v", err)
				continue
			}
			sender = gmail.FirstMessageSender(full)
			senderCache[threadID] = sender
		}
		senderCounts[sender]++
	}

	for sender, count := range senderCounts {
		u.metrics.SetLabelSender(labelID, labelName, sender, float64(count))
	}
}

func (u *Updater) getAllUnreadThreads(ctx context.Context, client gmail.API, labelID string) ([]string, error) {
	log.Infof("get_all_threads_for_label - this method can be expensive: %s", labelID)

	var threadIDs []string
	pageToken := ""

	for {
		res, err := client.ListUnreadThreads(ctx, labelID, pageToken)
		if err != nil {
			return nil, err
		}

		log.Infof("get_all_threads_for_label - result size estimate: %d", res.ResultSizeEstimate)

		for _, thread := range res.Threads {
			threadIDs = append(threadIDs, thread.Id)
		}

		if res.NextPageToken == "" {
			break
		}
		pageToken = res.NextPageToken
		log.Infof("Getting more threads for label %s: %d", labelID, len(threadIDs))
	}

	return threadIDs, nil
}

func (u *Updater) updateCustomQueries(ctx context.Context, client gmail.API) {
	queries := u.cfg.CustomQueries
	log.Infof("Updating custom message queries - starting (%d)", len(queries))

	for _, customQuery := range queries {
		log.Infof("Updating custom message queries: %s", customQuery.Name)

		res, err := client.ListMessages(ctx, customQuery.Query, "")
		if err != nil {
			log.Errorf("Error: %v", err)
			continue
		}

		u.metrics.SetCustomQuery(customQuery.Name, float64(res.ResultSizeEstimate))
	}

	log.Info("Updating gmail metrics - complete")
}
