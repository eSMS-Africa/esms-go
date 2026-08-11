# esms-go

Official Go SDK for the [eSMS Africa](https://esmsafrica.io) SMS API.

Send SMS across 14+ African countries, track delivery, schedule messages, and check your balance. Standard library only - no third-party dependencies.

## Install

```bash
go get github.com/eSMS-Africa/esms-go
```

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	esms "github.com/eSMS-Africa/esms-go"
)

func main() {
	client := esms.New(os.Getenv("ESMS_API_KEY"))

	res, err := client.Messages.Send(context.Background(), esms.SendParams{
		To:       "+256700000000",
		Text:     "Your verification code is 123456",
		SenderID: "eSMSAfrica", // optional - falls back to the route default
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res.ID, res.Status) // "...", "submitted"
}
```

Get an API key from the eSMS dashboard under **Developers → API Keys**. Live keys look like `esms_live_…`; test keys look like `esms_test_…`.

## Sending

```go
ctx := context.Background()

// Auto-detects the route (country) from the number.
client.Messages.Send(ctx, esms.SendParams{To: "+254711000000", Text: "Hi from Kenya"})

// Or pin a route explicitly.
client.Messages.Send(ctx, esms.SendParams{To: "+256700000000", Text: "Hi", Route: "ESMS_UG"})

// Schedule for later (5 minutes to 7 days out).
client.Messages.Schedule(ctx, esms.SendParams{
	To:          "+256700000000",
	Text:        "Reminder",
	ScheduledAt: "2026-08-01T09:00:00Z",
})
```

## Delivery status

```go
msg, _ := client.Messages.Get(ctx, res.ID)
fmt.Println(msg.Status) // queued | submitted | delivered | failed | ...
for _, e := range msg.Timeline {
	fmt.Println(e.At, e.Event)
}

// List recent messages
page, _ := client.Messages.List(ctx, esms.ListParams{Limit: 20, Status: "delivered"})
fmt.Println(page.Total)

// Retry a failed one
client.Messages.Retry(ctx, res.ID)
```

## Balance & routes

```go
bal, _ := client.Balance.Get(ctx)
fmt.Printf("%s %.2f\n", bal.Currency, bal.Balance)

routes, _ := client.Routes.List(ctx)
for _, r := range routes {
	fmt.Println(r.Code, r.CountryName, r.Currency, r.PricePerSegment)
}
```

## Errors

Every API failure is an `*esms.Error`. Use `errors.As` and the helper methods:

```go
import "errors"

res, err := client.Messages.Send(ctx, params)
if err != nil {
	var e *esms.Error
	if errors.As(err, &e) {
		switch {
		case e.IsInsufficientBalance():
			bal, _ := e.Balance()
			cost, _ := e.Cost()
			cur, _ := e.Currency()
			log.Printf("top up needed: have %.2f, need %.2f %s", bal, cost, cur)
		case e.IsAuthentication():
			log.Print("check your API key")
		default:
			log.Printf("%d %s: %s", e.Status, e.Code, e.Message)
		}
	}
}
```

| Helper | When |
|--------|------|
| `IsAuthentication()` | 401 - key missing or invalid |
| `IsPermission()` | 403 - not allowed |
| `IsNotFound()` | 404 - no such message |
| `IsInsufficientBalance()` | 402 - not enough credit (`Balance()`, `Cost()`, `Currency()`) |
| `IsRateLimit()` | 429 - slow down |
| `IsConnection()` | network failure or timeout |

## Configuration

```go
client := esms.New("esms_live_...",
	esms.WithBaseURL("https://sms.esmsafrica.io/api"), // default
	esms.WithMaxRetries(2),                            // transient failures with backoff
	esms.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
)
```

Requests take a `context.Context`, so timeouts and cancellation work as usual.

## License

MIT © eSMS Africa
