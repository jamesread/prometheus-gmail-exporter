package gmail

import (
	"context"

	gmailapi "google.golang.org/api/gmail/v1"
)

// MockAPI is a test double for gmail.API.
type MockAPI struct {
	Labels           []*gmailapi.Label
	ThreadsByLabel   map[string][]*gmailapi.Thread
	ThreadMetadata   map[string]*gmailapi.Thread
	MessageEstimates map[string]int64
}

func (m *MockAPI) ListLabels(ctx context.Context) ([]*gmailapi.Label, error) {
	return m.Labels, nil
}

func (m *MockAPI) GetLabel(ctx context.Context, id string) (*gmailapi.Label, error) {
	for _, label := range m.Labels {
		if label.Id == id {
			return label, nil
		}
	}
	return &gmailapi.Label{
		Id:            id,
		Name:          id,
		ThreadsTotal:  100,
		ThreadsUnread: 5,
	}, nil
}

func (m *MockAPI) ListUnreadThreads(ctx context.Context, labelID, pageToken string) (*gmailapi.ListThreadsResponse, error) {
	threads := m.ThreadsByLabel[labelID]
	return &gmailapi.ListThreadsResponse{
		Threads:           threads,
		ResultSizeEstimate: int64(len(threads)),
	}, nil
}

func (m *MockAPI) GetThreadMetadata(ctx context.Context, threadID string) (*gmailapi.Thread, error) {
	if thread, ok := m.ThreadMetadata[threadID]; ok {
		return thread, nil
	}
	return &gmailapi.Thread{
		Id: threadID,
		Messages: []*gmailapi.Message{
			{
				Payload: &gmailapi.MessagePart{
					Headers: []*gmailapi.MessagePartHeader{
						{Name: "From", Value: "sender@example.com"},
					},
				},
			},
		},
	}, nil
}

func (m *MockAPI) ListMessages(ctx context.Context, query, pageToken string) (*gmailapi.ListMessagesResponse, error) {
	estimate := m.MessageEstimates[query]
	return &gmailapi.ListMessagesResponse{ResultSizeEstimate: estimate}, nil
}
