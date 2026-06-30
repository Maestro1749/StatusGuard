package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type TelegramNotifier struct {
	token  string
	chatID int64
	client *http.Client
}

func NewTelegramNotifier(token string, chatID string) (*TelegramNotifier, error) {
	parsedChatID, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid telegram chat id: %w", err)
	}

	return &TelegramNotifier{
		token:  token,
		chatID: parsedChatID,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (n *TelegramNotifier) NotifyIncidentOpened(ctx context.Context, targetName string, targetURL string, errMsg string) error {
	text := fmt.Sprintf(
		"🔴Incident opened\n\nTarget: %s\nURL: %s\nError: %s",
		targetName,
		targetURL,
		errMsg,
	)

	return n.sendMessage(ctx, text)
}

func (n *TelegramNotifier) NotifyIncidentResolved(ctx context.Context, targetName string, targetURl string) error {
	text := fmt.Sprintf(
		"🟢Incident resolved\n\nTarget: %s\nURL: %s",
		targetName,
		targetURl,
	)

	return n.sendMessage(ctx, text)
}

func (n *TelegramNotifier) sendMessage(ctx context.Context, text string) error {
	body := map[string]any{
		"chat_id": n.chatID,
		"text":    text,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.token)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram api returned status %d", resp.StatusCode)
	}

	return nil
}
