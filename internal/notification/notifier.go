package notification

import "context"

type Notifier interface {
	NotifyIncidentOpened(ctx context.Context, targetName string, targetURL string, errMsg string) error
	NotifyIncidentResolved(ctx context.Context, targetName string, targetURL string) error
}
