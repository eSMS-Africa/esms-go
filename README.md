# esms-go

Official Go SDK for the [eSMS Africa](https://esmsafrica.io) SMS API.

Send SMS across Africa, track delivery, schedule messages, run managed OTP verification, and check your balance. Standard library only - no third-party dependencies.

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

// Price a message before sending (no charge, no delivery).
quote, _ := client.Messages.Rate(ctx, esms.RateParams{To: []string{"+256700000000"}, Text: "Hi"})

// Send to many numbers at once (inline recipients and/or contact lists).
batch, _ := client.Messages.SendBulk(ctx, esms.BulkSendParams{
	Recipients: []esms.BulkRecipient{{To: "+256700000000"}, {To: "+254711000000"}},
	Text:       "Hello from eSMS",
})
summary, _ := client.Messages.GetBatch(ctx, batch["batch_id"].(string), 0, 0)
```

`Messages.Send` attaches a random `Idempotency-Key` to every call so a retried request can never send or charge twice. Set `SendParams.IdempotencyKey` to make retries safe across process restarts too.

Advanced options: `Flash` (class-0 SMS shown on screen), `ValidityPeriod` (minutes, 1-4320), `Priority` (0-3) and `Binary: &esms.BinaryPayload{Data: hex, UDH: hex}` (leave `Text` empty). `BulkSendParams` also takes `Flash`, `ValidityPeriod`, `Priority` and `SkipUnavailable` (drop unreachable recipients, uncharged, instead of rejecting the whole batch).

## Personalised bulk + campaigns

Requires SDK 1.2.0 or later.

```go
// One call: a template with per-recipient merge fields. Invalid, duplicate and
// opted-out numbers come back in "rejected" and are never charged.
batch, err := client.Messages.SendBatch(ctx, esms.SendBatchParams{
	Template: "Hi {first_name|there}, your balance is UGX {balance}. Pay by {due_date}.",
	Recipients: []esms.BatchRecipient{
		{To: "+256772123456", Name: "Ada Obi", Vars: map[string]string{"balance": "1,500", "due_date": "1 Nov"}},
		{To: "0701234567", Vars: map[string]string{"balance": "200", "due_date": "1 Nov"}},
	},
	DefaultCountry: "UG",
	SenderID:       "ACMEBANK",
	Schedule:       &esms.Schedule{Mode: "scheduled", At: "2026-11-01T09:00", Timezone: "Africa/Kampala"},
})
status, _ := client.Messages.GetBatch(ctx, batch["batch_id"].(string), 0, 50)

// Big lists: upload a CSV / Excel file, map columns, then run a campaign.
data, _ := os.ReadFile("debtors.xlsx")
imp, _ := client.Imports.Upload(ctx, "debtors.xlsx", data, "UG")
job, _ := client.Imports.Process(ctx, imp["id"].(string), esms.ProcessParams{
	Mapping: esms.ImportMapping{Phone: "Mobile", Name: "Name", Fields: map[string]string{"Amount Due": "amount"}},
})
// poll client.Jobs.Get(ctx, int(job["job_id"].(float64))) until status is "done"
c, _ := client.Campaigns.Create(ctx, esms.JSON{"name": "October dues", "text": "Hi {first_name}, you owe {amount|0}.",
	"audience": []esms.JSON{{"type": "import", "id": imp["id"]}}}, "october-dues")
id := int(c["id"].(float64))
client.Campaigns.Prepare(ctx, id)
client.Campaigns.Launch(ctx, id)
report, _ := client.Campaigns.Report(ctx, id)
```

Also: `Campaigns.Pause/Resume/Cancel/RetryFailed/Duplicate/Recipients/Runs`, `Contacts`, `Groups`, `Templates` (with `StarterPack` and `UseStarter`).
The signed `campaign.completed` webhook uses the same `esms.VerifyWebhook`; decode it into `esms.CampaignEvent`.

## Delivery status

```go
msg, _ := client.Messages.Get(ctx, res.ID)
fmt.Println(msg.Status) // queued | submitted | delivered | failed | ...
for _, e := range msg.Timeline {
	fmt.Println(e.At, e.Event)
}

// List recent messages (filters: Status, To, BatchID, DateFrom, DateTo, Environment)
page, _ := client.Messages.List(ctx, esms.ListParams{Limit: 20, Status: "delivered"})
fmt.Println(page.Total)

// Status of up to 100 messages in one call
statuses, _ := client.Messages.Statuses(ctx, []string{res.ID})

// Retry a failed one -> *RetryResult{ID, Status, RetryCount}
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

## Verify (managed OTP)

```go
v, _ := client.Verify.Start(ctx, esms.VerifyStartParams{To: "+256700000000"}) // or AppID: "..."
// ...user types the code...
check, _ := client.Verify.Check(ctx, esms.VerifyCheckParams{
	VerificationID: v["verification_id"].(string),
	Code:           "123456",
})
if check["status"] == "approved" {
	// verified
}
```

Also available: `Verify.Get`, `Resend`, `Cancel`, `List`, and Verify Apps (`ListApps`, `CreateApp`, `GetApp`, `UpdateApp`, `DeleteApp`, `AppStats`).

## Opt-outs

```go
client.OptOuts.Add(ctx, "+256700000000")
optedOut, _ := client.OptOuts.List(ctx)
client.OptOuts.Remove(ctx, "+256700000000")
```

## Webhooks

Delivery-report webhooks are signed with HMAC-SHA256 in the `X-Webhook-Signature` header (`sha256=<hex>`). Verify against the exact raw body:

```go
ok := esms.VerifyWebhook(rawBody, r.Header.Get("X-Webhook-Signature"), secret)
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
| `IsInvalidRequest()` | 400 / 409 / 413 / 422 - bad request |
| `IsInsufficientBalance()` | 402 - not enough credit (`Balance()`, `Cost()`, `Currency()`) |
| `IsRateLimit()` | 429 - slow down |
| `IsServer()` | 5xx - server error |
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

Requests authenticate with `Authorization: Bearer <key>`. Non-idempotent POSTs (bulk sends, retries, OTP resends) are only retried on 429, never after a 5xx or network error, so they cannot run twice.

## License

MIT © eSMS Africa
