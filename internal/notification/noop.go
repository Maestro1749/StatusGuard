package notification

import "context"

type NoopNotifier struct{}

func NewNoopNotifier() *NoopNotifier {
	return &NoopNotifier{}
}

func (n *NoopNotifier) NotifyIncidentOpened(ctx context.Context, targetName string, targetURL string, errMsg string) error {
	return nil
}

func (h *NoopNotifier) NotifyIncidentResolved(ctx context.Context, targetName string, targetURL string) error {
	return nil
}
