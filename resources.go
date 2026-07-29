package esms

import (
	"context"
	"net/url"
)

// MessagesService handles SMS message operations.
type MessagesService struct{ c *Client }

// Send sends a single SMS.
func (s *MessagesService) Send(ctx context.Context, params SendParams) (*SendResult, error) {
	var out SendResult
	if err := s.c.do(ctx, "POST", "/messages/send", nil, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Schedule sends an SMS for later delivery (5 minutes to 7 days out).
// The ScheduleMode field is set automatically.
func (s *MessagesService) Schedule(ctx context.Context, params SendParams) (*SendResult, error) {
	params.ScheduleMode = "scheduled"
	return s.Send(ctx, params)
}

// List returns messages, most recent first.
func (s *MessagesService) List(ctx context.Context, params ListParams) (*MessageList, error) {
	q := url.Values{}
	if params.Page > 0 {
		q.Set("page", fmtInt(params.Page))
	}
	if params.Limit > 0 {
		q.Set("limit", fmtInt(params.Limit))
	}
	if params.Status != "" {
		q.Set("status", params.Status)
	}
	var out MessageList
	if err := s.c.do(ctx, "GET", "/messages", q, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a single message with its full delivery timeline.
func (s *MessagesService) Get(ctx context.Context, messageID string) (*Message, error) {
	var out Message
	if err := s.c.do(ctx, "GET", "/messages/"+url.PathEscape(messageID), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Retry retries a failed message.
func (s *MessagesService) Retry(ctx context.Context, messageID string) (*SendResult, error) {
	var out SendResult
	if err := s.c.do(ctx, "POST", "/messages/"+url.PathEscape(messageID)+"/retry", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SendBulk sends one message to every contact in the given contact lists.
func (s *MessagesService) SendBulk(ctx context.Context, params BulkSendParams) (map[string]any, error) {
	var out map[string]any
	if err := s.c.do(ctx, "POST", "/messages/send-bulk", nil, params, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// BalanceService reads the account balance.
type BalanceService struct{ c *Client }

// Get returns the current account balance and an SMS estimate.
func (s *BalanceService) Get(ctx context.Context) (*Balance, error) {
	var out Balance
	if err := s.c.do(ctx, "GET", "/balance", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RoutesService lists available routes.
type RoutesService struct{ c *Client }

// List returns all active routes (one per reachable country).
func (s *RoutesService) List(ctx context.Context) ([]Route, error) {
	var out []Route
	if err := s.c.do(ctx, "GET", "/routes", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
