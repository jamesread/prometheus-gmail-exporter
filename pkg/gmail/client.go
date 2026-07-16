package gmail

import (
	"context"

	gmailapi "google.golang.org/api/gmail/v1"
)

// LabelRef is a label ID reference for export.
type LabelRef struct {
	ID string
}

// API abstracts Gmail operations for testing.
type API interface {
	ListLabels(ctx context.Context) ([]*gmailapi.Label, error)
	GetLabel(ctx context.Context, id string) (*gmailapi.Label, error)
	ListUnreadThreads(ctx context.Context, labelID, pageToken string) (*gmailapi.ListThreadsResponse, error)
	GetThreadMetadata(ctx context.Context, threadID string) (*gmailapi.Thread, error)
	ListMessages(ctx context.Context, query, pageToken string) (*gmailapi.ListMessagesResponse, error)
}

// Client wraps the real Gmail API service.
type Client struct {
	svc *gmailapi.Service
}

func NewClient(svc *gmailapi.Service) *Client {
	return &Client{svc: svc}
}

func (c *Client) ListLabels(ctx context.Context) ([]*gmailapi.Label, error) {
	res, err := c.svc.Users.Labels.List("me").Context(ctx).Do()
	if err != nil {
		return nil, err
	}
	return res.Labels, nil
}

func (c *Client) GetLabel(ctx context.Context, id string) (*gmailapi.Label, error) {
	return c.svc.Users.Labels.Get("me", id).Context(ctx).Do()
}

func (c *Client) ListUnreadThreads(ctx context.Context, labelID, pageToken string) (*gmailapi.ListThreadsResponse, error) {
	call := c.svc.Users.Threads.List("me").LabelIds(labelID).Q("is:unread").Context(ctx)
	if pageToken != "" {
		call = call.PageToken(pageToken)
	}
	return call.Do()
}

func (c *Client) GetThreadMetadata(ctx context.Context, threadID string) (*gmailapi.Thread, error) {
	return c.svc.Users.Threads.Get("me", threadID).Format("metadata").Context(ctx).Do()
}

func (c *Client) ListMessages(ctx context.Context, query, pageToken string) (*gmailapi.ListMessagesResponse, error) {
	call := c.svc.Users.Messages.List("me").Q(query).Context(ctx)
	if pageToken != "" {
		call = call.PageToken(pageToken)
	}
	return call.Do()
}

// FirstMessageSender extracts the From header from the first message in a thread.
func FirstMessageSender(thread *gmailapi.Thread) string {
	if thread == nil || len(thread.Messages) == 0 {
		return "unknown-thread-no-messages"
	}
	first := thread.Messages[0]
	if first.Payload == nil {
		return "unknown-no-from"
	}
	for _, header := range first.Payload.Headers {
		if header.Name == "From" {
			return header.Value
		}
	}
	return "unknown-no-from"
}
