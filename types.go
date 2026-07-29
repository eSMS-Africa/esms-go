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
}

// SendResult is returned when a message is accepted.
type SendResult struct {
	ID           string  `json:"id"`
	Status       string  `json:"status"`
	Segments     int     `json:"segments"`
	Cost         float64 `json:"cost"`
	CostCurrency string  `json:"cost_currency"`
	RouteCost    float64 `json:"route_cost"`
	RouteCurr    string  `json:"route_currency"`
	Route        string  `json:"route"`
	BalanceAfter float64 `json:"balance_after"`
	ScheduledAt  *string `json:"scheduled_at"`
}

// ListParams filters a message listing.
type ListParams struct {
	Page   int    // zero-based page index
	Limit  int    // page size, 1-100
	Status string // optional status filter, e.g. "delivered"
}

// MessageSummary is a single row in a message listing.
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

// BulkSendParams sends one message to every contact in the given lists.
type BulkSendParams struct {
	ContactListIDs []int  `json:"contact_list_ids"`
	Text           string `json:"text"`
	SenderID       string `json:"sender_id,omitempty"`
	Route          string `json:"route,omitempty"`
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
