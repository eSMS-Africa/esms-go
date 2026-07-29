package esms

import "fmt"

// Error is returned for every failure that originates from the eSMS API.
// Inspect Status and Code to branch, or use errors.As with the typed helpers.
type Error struct {
	// Status is the HTTP status code (0 for connection failures).
	Status int
	// Code is the machine-readable error code, e.g. "insufficient_balance".
	Code string
	// Message is a human-readable description.
	Message string
	// Detail is the raw `detail` payload returned by the API.
	Detail any
	// RequestID is the X-Request-Id response header, useful for support.
	RequestID string
	// Err is the underlying cause for connection failures.
	Err error
}

func (e *Error) Error() string {
	switch {
	case e.Status > 0 && e.Code != "":
		return fmt.Sprintf("esms: HTTP %d %s: %s", e.Status, e.Code, e.Message)
	case e.Status > 0:
		return fmt.Sprintf("esms: HTTP %d: %s", e.Status, e.Message)
	default:
		return "esms: " + e.Message
	}
}

func (e *Error) Unwrap() error { return e.Err }

// IsAuthentication reports a 401 (missing or invalid API key).
func (e *Error) IsAuthentication() bool { return e.Status == 401 }

// IsPermission reports a 403 (authenticated but not allowed).
func (e *Error) IsPermission() bool { return e.Status == 403 }

// IsNotFound reports a 404.
func (e *Error) IsNotFound() bool { return e.Status == 404 }

// IsRateLimit reports a 429.
func (e *Error) IsRateLimit() bool { return e.Status == 429 }

// IsInsufficientBalance reports the 422 insufficient_balance error.
// When true, use Balance/Cost/Currency to read the shortfall.
func (e *Error) IsInsufficientBalance() bool { return e.Code == "insufficient_balance" }

// IsConnection reports a network/transport failure (never reached the API).
func (e *Error) IsConnection() bool { return e.Status == 0 }

// Balance returns the current balance from an insufficient_balance error.
func (e *Error) Balance() (float64, bool) { return e.detailFloat("balance") }

// Cost returns the required cost from an insufficient_balance error.
func (e *Error) Cost() (float64, bool) { return e.detailFloat("cost") }

// Currency returns the currency from an insufficient_balance error.
func (e *Error) Currency() (string, bool) {
	m, ok := e.Detail.(map[string]any)
	if !ok {
		return "", false
	}
	s, ok := m["currency"].(string)
	return s, ok
}

func (e *Error) detailFloat(key string) (float64, bool) {
	m, ok := e.Detail.(map[string]any)
	if !ok {
		return 0, false
	}
	f, ok := m[key].(float64)
	return f, ok
}

// errorFromResponse builds an *Error from an HTTP status and parsed body.
func errorFromResponse(status int, body any, requestID string) *Error {
	var detail any = body
	if m, ok := body.(map[string]any); ok {
		if d, has := m["detail"]; has {
			detail = d
		}
	}

	var code, message string
	switch d := detail.(type) {
	case map[string]any:
		if c, ok := d["code"].(string); ok {
			code = c
		}
		if msg, ok := d["message"].(string); ok {
			message = msg
		}
	case string:
		message = d
	}
	if message == "" {
		message = fmt.Sprintf("HTTP %d", status)
	}

	return &Error{
		Status:    status,
		Code:      code,
		Message:   message,
		Detail:    detail,
		RequestID: requestID,
	}
}
