package esms

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	client := New("esms_test_abc", WithBaseURL(srv.URL), WithMaxRetries(0))
	return client, srv
}

func TestNewRequiresKey(t *testing.T) {
	c := New("")
	_, err := c.Balance.Get(context.Background())
	if err == nil {
		t.Fatal("expected an error for a missing API key")
	}
}

func TestDefaultBaseURL(t *testing.T) {
	if got := New("k").BaseURL(); got != DefaultBaseURL {
		t.Fatalf("base URL = %q, want %q", got, DefaultBaseURL)
	}
}

func TestSend(t *testing.T) {
	var gotBody SendParams
	var gotAuth string
	client, srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages/send" || r.Method != "POST" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","status":"submitted","cost_currency":"KES","balance_after":9.6}`))
	})
	defer srv.Close()

	res, err := client.Messages.Send(context.Background(), SendParams{To: "+256700000000", Text: "Hi"})
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "msg_1" || res.Status != "submitted" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.CostCurrency != "KES" || res.BalanceAfter != 9.6 {
		t.Fatalf("mapping wrong: %+v", res)
	}
	if gotBody.To != "+256700000000" {
		t.Fatalf("body.to = %q", gotBody.To)
	}
	if gotAuth != "Bearer esms_test_abc" {
		t.Fatalf("auth = %q", gotAuth)
	}
}

func TestScheduleSetsMode(t *testing.T) {
	var body SendParams
	client, srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"id":"m","status":"scheduled"}`))
	})
	defer srv.Close()

	_, err := client.Messages.Schedule(context.Background(), SendParams{
		To: "+256700000000", Text: "x", ScheduledAt: "2026-08-01T09:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if body.ScheduleMode != "scheduled" {
		t.Fatalf("schedule_mode = %q", body.ScheduleMode)
	}
}

func TestAuthError(t *testing.T) {
	client, srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"detail":"Not authenticated"}`))
	})
	defer srv.Close()

	_, err := client.Messages.List(context.Background(), ListParams{})
	e, ok := err.(*Error)
	if !ok || !e.IsAuthentication() {
		t.Fatalf("want AuthenticationError, got %v", err)
	}
}

func TestInsufficientBalance(t *testing.T) {
	client, srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		_, _ = w.Write([]byte(`{"detail":{"code":"insufficient_balance","message":"low","balance":1,"cost":5,"currency":"KES"}}`))
	})
	defer srv.Close()

	_, err := client.Messages.Send(context.Background(), SendParams{To: "+256700000000", Text: "x"})
	e, ok := err.(*Error)
	if !ok || !e.IsInsufficientBalance() {
		t.Fatalf("want insufficient_balance, got %v", err)
	}
	if bal, ok := e.Balance(); !ok || bal != 1 {
		t.Fatalf("balance = %v", bal)
	}
	if cost, ok := e.Cost(); !ok || cost != 5 {
		t.Fatalf("cost = %v", cost)
	}
	if cur, ok := e.Currency(); !ok || cur != "KES" {
		t.Fatalf("currency = %v", cur)
	}
}

func TestNotFound(t *testing.T) {
	client, srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"detail":"Message not found"}`))
	})
	defer srv.Close()

	_, err := client.Messages.Get(context.Background(), "nope")
	e, ok := err.(*Error)
	if !ok || !e.IsNotFound() {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestConnectionError(t *testing.T) {
	// Point at a closed port to force a transport failure.
	client := New("k", WithBaseURL("http://127.0.0.1:1"), WithMaxRetries(0))
	_, err := client.Balance.Get(context.Background())
	e, ok := err.(*Error)
	if !ok || !e.IsConnection() {
		t.Fatalf("want connection error, got %v", err)
	}
}

func TestRoutes(t *testing.T) {
	client, srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"code":"ESMS_UG","country_name":"Uganda","price_per_segment":35,"is_active":true}]`))
	})
	defer srv.Close()

	routes, err := client.Routes.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes[0].Code != "ESMS_UG" || routes[0].PricePerSegment != 35 || !routes[0].IsActive {
		t.Fatalf("unexpected routes: %+v", routes)
	}
}
