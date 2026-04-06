package webhook

import "context"

type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type Publisher interface {
	Publish(ctx context.Context, event Event) error
}

type NoopPublisher struct{}

func (NoopPublisher) Publish(_ context.Context, _ Event) error {
	return nil
}
