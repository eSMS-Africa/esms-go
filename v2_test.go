package esms

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type seenReq struct {
	method, path, query, ctype, idem string
	body                             []byte
}

func recorder(t *testing.T) (*Client, *[]seenReq) {
	t.Helper()
	var seen []seenReq
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen = append(seen, seenReq{r.Method, r.URL.EscapedPath(), r.URL.RawQuery, r.Header.Get("Content-Type"), r.Header.Get("Idempotency-Key"), b})
		if r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/contacts/") {
			w.WriteHeader(204)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1,"batch_id":"b1"}`))
	})
	return c, &seen
}

func TestSendBatch(t *testing.T) {
	c, seen := recorder(t)
	out, err := c.Messages.SendBatch(context.Background(), SendBatchParams{
		Template:          "Hi {first_name}, owe {amount}",
		Recipients:        []BatchRecipient{{To: "+256772123456", Vars: map[string]string{"amount": "1,500"}}},
		Schedule:          &Schedule{Mode: "scheduled", At: "2026-11-01T09:00", Timezone: "Africa/Kampala"},
		ThrottlePerMinute: 100,
	})
	if err != nil || out["batch_id"] != "b1" {
		t.Fatalf("err=%v out=%v", err, out)
	}
	s := (*seen)[0]
	if s.path != "/messages/batch" || len(s.idem) != 36 {
		t.Fatalf("bad request %+v", s)
	}
	var body map[string]any
	_ = json.Unmarshal(s.body, &body)
	if body["throttle_per_minute"].(float64) != 100 || body["schedule"].(map[string]any)["timezone"] != "Africa/Kampala" {
		t.Fatalf("body %v", body)
	}
	if _, ok := body["IdempotencyKey"]; ok {
		t.Fatal("idempotency key leaked into body")
	}
}

func TestCampaignPaths(t *testing.T) {
	c, seen := recorder(t)
	ctx := context.Background()
	_, _ = c.Campaigns.Create(ctx, JSON{"name": "Dues", "text": "Hi"}, "c-1")
	_, _ = c.Campaigns.Prepare(ctx, 3)
	_, _ = c.Campaigns.Review(ctx, 3)
	_, _ = c.Campaigns.Launch(ctx, 3)
	_, _ = c.Campaigns.Approve(ctx, 3, "ok")
	_, _ = c.Campaigns.RetryFailed(ctx, 3)
	_, _ = c.Campaigns.Recipients(ctx, 3, ListQuery{"status": "failed"})
	var got []string
	for _, s := range *seen {
		got = append(got, s.method+" "+s.path)
	}
	want := "POST /campaigns|POST /campaigns/3/prepare|GET /campaigns/3/review|POST /campaigns/3/launch|POST /campaigns/3/approve|POST /campaigns/3/retry-failed|GET /campaigns/3/recipients"
	if strings.Join(got, "|") != want {
		t.Fatalf("got %v", got)
	}
	if (*seen)[0].idem != "c-1" || (*seen)[6].query != "status=failed" || string((*seen)[4].body) != `{"note":"ok"}` {
		t.Fatalf("details %+v", *seen)
	}
}

func TestImportUploadMultipart(t *testing.T) {
	c, seen := recorder(t)
	_, err := c.Imports.Upload(context.Background(), "a.csv", []byte("phone\n0772123456\n"), "UG")
	if err != nil {
		t.Fatal(err)
	}
	s := (*seen)[0]
	if !strings.HasPrefix(s.ctype, "multipart/form-data; boundary=") || !strings.Contains(string(s.body), `filename="a.csv"`) ||
		!strings.Contains(string(s.body), "0772123456") || !strings.Contains(string(s.body), `name="default_country"`) {
		t.Fatalf("multipart %+v", s)
	}
}

func TestContactsGroupsTemplates(t *testing.T) {
	c, seen := recorder(t)
	ctx := context.Background()
	_, _ = c.Contacts.Get(ctx, "+256772123456")
	if err := c.Contacts.Delete(ctx, "+256772123456"); err != nil {
		t.Fatal(err)
	}
	_, _ = c.Groups.AddContacts(ctx, 5, []GroupContact{{Phone: "0772123456"}}, "UG")
	_, _ = c.Templates.UseStarter(ctx, "payment-reminder")
	if (*seen)[0].path != "/contacts/+256772123456" && (*seen)[0].path != "/contacts/%2B256772123456" {
		t.Fatalf("path %s", (*seen)[0].path)
	}
	if !strings.Contains(string((*seen)[2].body), `"default_country":"UG"`) || (*seen)[3].path != "/templates/starter-pack/payment-reminder" {
		t.Fatalf("details %+v", *seen)
	}
}

func TestCampaignWebhook(t *testing.T) {
	body := []byte(`{"event":"campaign.completed","campaign_id":1}`)
	m := hmac.New(sha256.New, []byte("whsec_x"))
	m.Write(body)
	if !VerifyWebhook(body, "sha256="+hex.EncodeToString(m.Sum(nil)), "whsec_x") {
		t.Fatal("signature should verify")
	}
	var ev CampaignEvent
	if err := json.Unmarshal(body, &ev); err != nil || ev.CampaignID != 1 {
		t.Fatal("decode")
	}
}
