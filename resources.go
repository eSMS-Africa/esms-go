package esms

import (
	"context"
	"net/url"
	"strconv"
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

// VerifyService sends and checks one-time verification codes (managed OTP).
type VerifyService struct{ c *Client }

// VerifyStartParams configures a verification.
type VerifyStartParams struct {
	To            string `json:"to"`
	AppID         string `json:"app_id,omitempty"` // a Verify App id - its template/policy apply
	SenderID      string `json:"sender_id,omitempty"`
	CodeLength    int    `json:"code_length,omitempty"`
	ExpirySeconds int    `json:"expiry_seconds,omitempty"`
	Template      string `json:"template,omitempty"`
	// IdempotencyKey, when set, makes a retried Start return the original verification.
	IdempotencyKey string `json:"-"`
}

// Start sends a verification code to a phone number.
func (s *VerifyService) Start(ctx context.Context, params VerifyStartParams) (map[string]any, error) {
	var out map[string]any
	var headers map[string]string
	if params.IdempotencyKey != "" {
		headers = map[string]string{"Idempotency-Key": params.IdempotencyKey}
	}
	if err := s.c.do(ctx, "POST", "/verify/start", nil, params, &out, headers); err != nil {
		return nil, err
	}
	return out, nil
}

// VerifyCheckParams checks a code the user entered.
type VerifyCheckParams struct {
	Code           string `json:"code"`
	VerificationID string `json:"verification_id,omitempty"`
	To             string `json:"to,omitempty"`
}

// Check validates a code. The returned map includes status: approved|pending|failed|expired.
func (s *VerifyService) Check(ctx context.Context, params VerifyCheckParams) (map[string]any, error) {
	var out map[string]any
	if err := s.c.do(ctx, "POST", "/verify/check", nil, params, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get fetches a verification's status without consuming an attempt.
func (s *VerifyService) Get(ctx context.Context, id string) (map[string]any, error) {
	var out map[string]any
	if err := s.c.do(ctx, "GET", "/verify/"+id, nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Resend sends a fresh code for the same verification.
func (s *VerifyService) Resend(ctx context.Context, id string) (map[string]any, error) {
	var out map[string]any
	if err := s.c.do(ctx, "POST", "/verify/"+id+"/resend", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Cancel voids an in-flight verification.
func (s *VerifyService) Cancel(ctx context.Context, id string) (map[string]any, error) {
	var out map[string]any
	if err := s.c.do(ctx, "POST", "/verify/"+id+"/cancel", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// VerifyListParams filters a verification listing.
type VerifyListParams struct {
	Status string
	AppID  string
	To     string
	Page   int
	Limit  int
}

// List returns your verifications (most recent first).
func (s *VerifyService) List(ctx context.Context, params VerifyListParams) (map[string]any, error) {
	q := url.Values{}
	if params.Status != "" {
		q.Set("status", params.Status)
	}
	if params.AppID != "" {
		q.Set("app_id", params.AppID)
	}
	if params.To != "" {
		q.Set("to", params.To)
	}
	if params.Page > 0 {
		q.Set("page", strconv.Itoa(params.Page))
	}
	if params.Limit > 0 {
		q.Set("limit", strconv.Itoa(params.Limit))
	}
	var out map[string]any
	if err := s.c.do(ctx, "GET", "/verify", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ---- Verify Apps ----

// ListApps returns your Verify Apps.
func (s *VerifyService) ListApps(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := s.c.do(ctx, "GET", "/verify/apps", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateApp creates a Verify App (name, sender_id, template, code_length, expiry_seconds, max_attempts, allowed_countries, locale, enabled).
func (s *VerifyService) CreateApp(ctx context.Context, body map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := s.c.do(ctx, "POST", "/verify/apps", nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetApp fetches one Verify App.
func (s *VerifyService) GetApp(ctx context.Context, id string) (map[string]any, error) {
	var out map[string]any
	if err := s.c.do(ctx, "GET", "/verify/apps/"+id, nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateApp updates a Verify App (full replace - send all fields; name is required).
func (s *VerifyService) UpdateApp(ctx context.Context, id string, body map[string]any) (map[string]any, error) {
	var out map[string]any
	if err := s.c.do(ctx, "PATCH", "/verify/apps/"+id, nil, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteApp deletes a Verify App.
func (s *VerifyService) DeleteApp(ctx context.Context, id string) error {
	return s.c.do(ctx, "DELETE", "/verify/apps/"+id, nil, nil, nil)
}

// AppStats returns per-app verification stats.
func (s *VerifyService) AppStats(ctx context.Context, id string, days int) (map[string]any, error) {
	q := url.Values{}
	if days > 0 {
		q.Set("days", strconv.Itoa(days))
	}
	var out map[string]any
	if err := s.c.do(ctx, "GET", "/verify/apps/"+id+"/stats", q, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// OptOutsService manages the opt-out (STOP / DND) list.
type OptOutsService struct{ c *Client }

// List returns numbers that have opted out of your messages.
func (s *OptOutsService) List(ctx context.Context) ([]map[string]any, error) {
	var out struct {
		OptOuts []map[string]any `json:"opt_outs"`
	}
	if err := s.c.do(ctx, "GET", "/opt-outs", nil, nil, &out); err != nil {
		return nil, err
	}
	return out.OptOuts, nil
}

// Add manually adds a number to your opt-out list.
func (s *OptOutsService) Add(ctx context.Context, phone string) (map[string]any, error) {
	var out map[string]any
	if err := s.c.do(ctx, "POST", "/opt-outs", nil, map[string]string{"phone": phone}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Remove takes a number off your opt-out list.
func (s *OptOutsService) Remove(ctx context.Context, phone string) (map[string]any, error) {
	var out map[string]any
	if err := s.c.do(ctx, "DELETE", "/opt-outs/"+url.PathEscape(phone), nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
