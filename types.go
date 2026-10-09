package esms

// SendParams describes a single SMS to send.
type SendParams struct {
	// To is the recipient in international format, e.g. "+256700000000".
	To string `json:"to"`
	// Text is the message body (Unicode supported).
	Text string `json:"text"`
	// SenderID is an approved sender ID; defaults to the route's default.
	SenderID string `json:"sender_id,omitempty"`
	// Route pins an explicit route code such as "ESMS_UG".
	Route string `json:"route,omitempty"`
	// ScheduleMode is "now" (default) or "scheduled".
	ScheduleMode string `json:"schedule_mode,omitempty"`
	// ScheduledAt is an ISO-8601 UTC time; required when ScheduleMode is "scheduled".
	ScheduledAt string `json:"scheduled_at,omitempty"`
	// International bills the international-delivery rate (USD) for this send.
	International bool `json:"international,omitempty"`
	// IdempotencyKey makes the send safe to retry: a repeat with the same key
	// returns the original result. A random key is generated when empty.
	IdempotencyKey string `json:"-"`
}

// SendResult is returned when a message is accepted. BalanceAfter is 0 for
// sandbox sends and idempotent replays (the API omits it there).
type SendResult struct {
	ID           string  `json:"id"`
	Status       string  `json:"status"`
	Segments     int     `json:"segments"`
	Encoding     string  `json:"encoding"`
	Cost         float64 `json:"cost"`
	CostCurrency string  `json:"cost_currency"`
	RouteCost    float64 `json:"route_cost"`
	RouteCurr    string  `json:"route_currency"`
	Route        string  `json:"route"`
	BalanceAfter float64 `json:"balance_after"`
	ScheduledAt  *string `json:"scheduled_at"`
}

// RetryResult is returned by Messages.Retry.
type RetryResult struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	RetryCount int    `json:"retry_count"`
}

// ListParams filters a message listing.
type ListParams struct {
	Page        int    // zero-based page index
	Limit       int    // page size, 1-100
	Status      string // optional status filter, e.g. "delivered"
	To          string // recipient phone (E.164)
	BatchID     string // bulk batch id from SendBulk
	DateFrom    string // ISO-8601 lower bound on created time
	DateTo      string // ISO-8601 upper bound on created time
	Environment string // "live" (default), "test" or "all"
}

// MessageSummary is a single row in a message listing. Text is truncated to
// 100 characters in listings; use Messages.Get for the full body.
type MessageSummary struct {
	ID          string  `json:"id"`
	Phone       string  `json:"phone"`
	Text        string  `json:"text"`
	SenderID    *string `json:"sender_id"`
	Route       *string `json:"route"`
	Country     *string `json:"country"`
	Segments    int     `json:"segments"`
	Cost        float64 `json:"cost"`
	Currency    *string `json:"currency"`
	Status      string  `json:"status"`
	ErrorCode   *string `json:"error_code"`
	RetryCount  int     `json:"retry_count"`
	CreatedAt   string  `json:"created_at"`
	DeliveredAt *string `json:"delivered_at"`
}

// MessageList is a page of messages.
type MessageList struct {
	Messages []MessageSummary `json:"messages"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	Limit    int              `json:"limit"`
}

// TimelineEvent is one entry in a message's delivery history.
type TimelineEvent struct {
	Event    string         `json:"event"`
	Status   string         `json:"status"`
	Detail   *string        `json:"detail"`
	At       string         `json:"at"`
	Metadata map[string]any `json:"metadata"`
}

// Message is a single message with its full delivery timeline.
type Message struct {
	MessageSummary
	ErrorMessage *string         `json:"error_message"`
	SubmittedAt  *string         `json:"submitted_at"`
	FailedAt     *string         `json:"failed_at"`
	Timeline     []TimelineEvent `json:"timeline"`
}

// BulkRecipient is an inline bulk recipient.
type BulkRecipient struct {
	To   string         `json:"to"`
	Name string         `json:"name,omitempty"`
	Vars map[string]any `json:"vars,omitempty"`
}

// BulkSendParams sends one message to many recipients: contact lists and/or
// inline Recipients.
type BulkSendParams struct {
	ContactListIDs []int           `json:"contact_list_ids,omitempty"`
	Recipients     []BulkRecipient `json:"recipients,omitempty"`
	Text           string          `json:"text"`
	SenderID       string          `json:"sender_id,omitempty"`
	Route          string          `json:"route,omitempty"`
	// ScheduleMode is "now" (default), "scheduled" (contact lists only) or "drip".
	ScheduleMode string `json:"schedule_mode,omitempty"`
	ScheduledAt  string `json:"scheduled_at,omitempty"`
	// DripRate is messages per minute for ScheduleMode "drip".
	DripRate      int  `json:"drip_rate,omitempty"`
	International bool `json:"international,omitempty"`
}

// RateParams prices a message before sending.
type RateParams struct {
	To            []string `json:"to"`
	Text          string   `json:"text"`
	Route         string   `json:"route,omitempty"`
	International bool     `json:"international,omitempty"`
}

// Balance is the current account balance.
type Balance struct {
	Balance     float64 `json:"balance"`
	Currency    string  `json:"currency"`
	SMSEstimate *int    `json:"sms_estimate"`
}

// Route is an active SMS route (one per reachable country).
type Route struct {
	Code            string  `json:"code"`
	Name            string  `json:"name"`
	CountryCode     string  `json:"country_code"`
	CountryName     string  `json:"country_name"`
	Currency        string  `json:"currency"`
	PricePerSegment float64 `json:"price_per_segment"`
	SenderIDDefault string  `json:"sender_id_default"`
	IsActive        bool    `json:"is_active"`
}
