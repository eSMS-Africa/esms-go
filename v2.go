package esms

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/url"
	"strconv"
)

// rawBody is sent as-is (multipart uploads).
type rawBody struct {
	data        []byte
	contentType string
}

// JSON is a raw API object (snake_case keys).
type JSON = map[string]any

// Schedule is when a batch or campaign sends.
type Schedule struct {
	Mode       string         `json:"mode"`                 // now | scheduled | recurring
	At         string         `json:"at,omitempty"`         // ISO 8601; without an offset it is read in Timezone
	Timezone   string         `json:"timezone,omitempty"`   // IANA name, e.g. Africa/Nairobi
	Recurrence map[string]any `json:"recurrence,omitempty"` // {freq, time, interval, weekdays, monthday, cron, until, count}
}

// BatchRecipient is one recipient of a personalised batch.
type BatchRecipient struct {
	To   string            `json:"to"`
	Name string            `json:"name,omitempty"`
	Vars map[string]string `json:"vars,omitempty"` // merge-field values
	Text string            `json:"text,omitempty"` // a full message (instead of the template)
}

// SendBatchParams configures POST /v1/messages/batch.
type SendBatchParams struct {
	Template          string           `json:"template,omitempty"`
	TemplateID        int              `json:"template_id,omitempty"`
	Recipients        []BatchRecipient `json:"recipients,omitempty"`
	Messages          []BatchRecipient `json:"messages,omitempty"` // full messages {to, text}
	SenderID          string           `json:"sender_id,omitempty"`
	SenderFallback    *bool            `json:"sender_fallback,omitempty"`
	MessageType       string           `json:"message_type,omitempty"` // transactional | promotional
	Name              string           `json:"name,omitempty"`
	Tags              []string         `json:"tags,omitempty"`
	DefaultCountry    string           `json:"default_country,omitempty"`
	Schedule          *Schedule        `json:"schedule,omitempty"`
	ThrottlePerMinute int              `json:"throttle_per_minute,omitempty"`
	CallbackURL       string           `json:"callback_url,omitempty"`
	// IdempotencyKey makes a retried call return the original batch; a random
	// key is generated when empty, so a retry never sends twice.
	IdempotencyKey string `json:"-"`
}

// CampaignEvent is the signed campaign.completed webhook body (check it with VerifyWebhook).
type CampaignEvent struct {
	Event       string   `json:"event"`
	CampaignID  int      `json:"campaign_id"`
	BatchID     string   `json:"batch_id"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Environment string   `json:"environment"`
	Livemode    bool     `json:"livemode"`
	Recipients  int      `json:"recipients"`
	Sent        int      `json:"sent"`
	Failed      int      `json:"failed"`
	Skipped     int      `json:"skipped"`
	Spent       float64  `json:"spent"`
	Currency    string   `json:"currency"`
	Tags        []string `json:"tags"`
	StartedAt   string   `json:"started_at"`
	CompletedAt string   `json:"completed_at"`
	ReportURL   string   `json:"report_url"`
}

// SendBatch sends personalised messages to many recipients (durable,
// resumable). Rejected numbers (invalid, duplicate, opted out) are listed in
// the response and never charged.
func (s *MessagesService) SendBatch(ctx context.Context, params SendBatchParams) (JSON, error) {
	key := params.IdempotencyKey
	if key == "" {
		key = newIdempotencyKey()
	}
	var out JSON
	err := s.c.do(ctx, "POST", "/messages/batch", nil, params, &out, map[string]string{"Idempotency-Key": key})
	return out, err
}

func (c *Client) json(ctx context.Context, method, path string, q url.Values, body any, headers ...map[string]string) (JSON, error) {
	var out JSON
	err := c.do(ctx, method, path, q, body, &out, headers...)
	return out, err
}

func idp(id int) string { return strconv.Itoa(id) }

func query(kv map[string]string) url.Values {
	q := url.Values{}
	for k, v := range kv {
		if v != "" {
			q.Set(k, v)
		}
	}
	return q
}

// ListQuery is a generic filter set (e.g. status, q, page, limit).
type ListQuery map[string]string

// CampaignsService manages campaigns: draft -> preview / prepare / review -> test -> launch.
type CampaignsService struct{ c *Client }

// Create creates a draft campaign (body uses API field names; "launch": true creates and launches).
func (s *CampaignsService) Create(ctx context.Context, body JSON, idempotencyKey string) (JSON, error) {
	return s.c.json(ctx, "POST", "/campaigns", nil, body, map[string]string{"Idempotency-Key": idempotencyKey})
}

// List lists campaigns (filters: status, q, tag, page, limit, include_runs).
func (s *CampaignsService) List(ctx context.Context, f ListQuery) (JSON, error) {
	return s.c.json(ctx, "GET", "/campaigns", query(f), nil)
}

// Get fetches one campaign.
func (s *CampaignsService) Get(ctx context.Context, id int) (JSON, error) {
	return s.c.json(ctx, "GET", "/campaigns/"+idp(id), nil, nil)
}

// Update partially updates a draft.
func (s *CampaignsService) Update(ctx context.Context, id int, body JSON) (JSON, error) {
	return s.c.json(ctx, "PATCH", "/campaigns/"+idp(id), nil, body)
}

// Delete deletes a campaign that has not sent.
func (s *CampaignsService) Delete(ctx context.Context, id int) (JSON, error) {
	return s.c.json(ctx, "DELETE", "/campaigns/"+idp(id), nil, nil)
}

// Preview renders the first n messages and the longest-message counter.
func (s *CampaignsService) Preview(ctx context.Context, id int, text string, n int) (JSON, error) {
	body := JSON{}
	if text != "" {
		body["text"] = text
	}
	if n > 0 {
		body["n"] = n
	}
	return s.post(ctx, id, "preview", body)
}

// Prepare builds the recipient list and the exact estimate (status ready | preparing).
func (s *CampaignsService) Prepare(ctx context.Context, id int) (JSON, error) {
	return s.post(ctx, id, "prepare", nil)
}

// Review returns blocking problems, warnings and the estimate.
func (s *CampaignsService) Review(ctx context.Context, id int) (JSON, error) {
	return s.c.json(ctx, "GET", "/campaigns/"+idp(id)+"/review", nil, nil)
}

// Test sends the first rendered message to up to 3 of your own numbers.
func (s *CampaignsService) Test(ctx context.Context, id int, to []string) (JSON, error) {
	return s.post(ctx, id, "test", JSON{"to": to})
}

// Launch sends now, schedules, or submits for approval.
func (s *CampaignsService) Launch(ctx context.Context, id int) (JSON, error) {
	return s.post(ctx, id, "launch", nil)
}

// Approve approves a campaign waiting for approval.
func (s *CampaignsService) Approve(ctx context.Context, id int, note string) (JSON, error) {
	return s.post(ctx, id, "approve", noteBody(note))
}

// Reject sends a campaign back to draft.
func (s *CampaignsService) Reject(ctx context.Context, id int, note string) (JSON, error) {
	return s.post(ctx, id, "reject", noteBody(note))
}

// Pause pauses a scheduled or sending campaign.
func (s *CampaignsService) Pause(ctx context.Context, id int) (JSON, error) {
	return s.post(ctx, id, "pause", nil)
}

// Resume resumes a paused campaign.
func (s *CampaignsService) Resume(ctx context.Context, id int) (JSON, error) {
	return s.post(ctx, id, "resume", nil)
}

// Cancel cancels a campaign (in-flight messages finish).
func (s *CampaignsService) Cancel(ctx context.Context, id int) (JSON, error) {
	return s.post(ctx, id, "cancel", nil)
}

// RetryFailed sends again to recipients whose message failed (charged like a new send).
func (s *CampaignsService) RetryFailed(ctx context.Context, id int) (JSON, error) {
	return s.post(ctx, id, "retry-failed", nil)
}

// Duplicate copies a campaign into a new draft.
func (s *CampaignsService) Duplicate(ctx context.Context, id int) (JSON, error) {
	return s.post(ctx, id, "duplicate", nil)
}

// Report returns totals, delivery rate, cost, timing, failure reasons and breakdowns.
func (s *CampaignsService) Report(ctx context.Context, id int) (JSON, error) {
	return s.c.json(ctx, "GET", "/campaigns/"+idp(id)+"/report", nil, nil)
}

// Stats returns compact live status counts.
func (s *CampaignsService) Stats(ctx context.Context, id int) (JSON, error) {
	return s.c.json(ctx, "GET", "/campaigns/"+idp(id)+"/stats", nil, nil)
}

// Recipients lists per-recipient status (filters: status, country, q, page, limit).
func (s *CampaignsService) Recipients(ctx context.Context, id int, f ListQuery) (JSON, error) {
	return s.c.json(ctx, "GET", "/campaigns/"+idp(id)+"/recipients", query(f), nil)
}

// Runs lists the runs of a recurring campaign.
func (s *CampaignsService) Runs(ctx context.Context, id int) (JSON, error) {
	return s.c.json(ctx, "GET", "/campaigns/"+idp(id)+"/runs", nil, nil)
}

func (s *CampaignsService) post(ctx context.Context, id int, action string, body any) (JSON, error) {
	return s.c.json(ctx, "POST", "/campaigns/"+idp(id)+"/"+action, nil, body)
}

func noteBody(note string) any {
	if note == "" {
		return nil
	}
	return JSON{"note": note}
}

// ImportsService uploads and validates recipient files.
type ImportsService struct{ c *Client }

// Upload uploads a CSV / XLSX / XLS file (filename tells the type).
func (s *ImportsService) Upload(ctx context.Context, filename string, data []byte, defaultCountry string) (JSON, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if defaultCountry != "" {
		if err := w.WriteField("default_country", defaultCountry); err != nil {
			return nil, err
		}
	}
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	if _, err := fw.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return s.c.json(ctx, "POST", "/imports", nil, rawBody{buf.Bytes(), w.FormDataContentType()})
}

// Paste imports pasted numbers (or tab-separated rows with a header).
func (s *ImportsService) Paste(ctx context.Context, text, defaultCountry string) (JSON, error) {
	return s.c.json(ctx, "POST", "/imports/paste", nil, omit(JSON{"text": text, "default_country": defaultCountry}))
}

// GSheet imports a shared Google Sheet.
func (s *ImportsService) GSheet(ctx context.Context, sheetURL, defaultCountry string) (JSON, error) {
	return s.c.json(ctx, "POST", "/imports/gsheet", nil, omit(JSON{"url": sheetURL, "default_country": defaultCountry}))
}

// Get returns an import's status and counts.
func (s *ImportsService) Get(ctx context.Context, id string) (JSON, error) {
	return s.c.json(ctx, "GET", "/imports/"+url.PathEscape(id), nil, nil)
}

// ImportMapping maps file columns.
type ImportMapping struct {
	Phone  string            `json:"phone"`
	Name   string            `json:"name,omitempty"`
	Fields map[string]string `json:"fields,omitempty"` // column -> merge-field key
}

// ProcessParams starts validation.
type ProcessParams struct {
	Mapping        ImportMapping `json:"mapping"`
	DefaultCountry string        `json:"default_country,omitempty"`
	Dedupe         *bool         `json:"dedupe,omitempty"`
	SkipOptedOut   *bool         `json:"skip_opted_out,omitempty"`
}

// Process validates the import in the background (poll Jobs.Get or Get).
func (s *ImportsService) Process(ctx context.Context, id string, p ProcessParams) (JSON, error) {
	return s.c.json(ctx, "POST", "/imports/"+url.PathEscape(id)+"/process", nil, p)
}

// Errors lists rows that were not imported (filters: reason, page, limit).
func (s *ImportsService) Errors(ctx context.Context, id string, f ListQuery) (JSON, error) {
	return s.c.json(ctx, "GET", "/imports/"+url.PathEscape(id)+"/errors", query(f), nil)
}

// Recipients lists the valid recipients.
func (s *ImportsService) Recipients(ctx context.Context, id string, f ListQuery) (JSON, error) {
	return s.c.json(ctx, "GET", "/imports/"+url.PathEscape(id)+"/recipients", query(f), nil)
}

// SaveToGroup saves the valid recipients into a group (body: group_id or name, tags).
func (s *ImportsService) SaveToGroup(ctx context.Context, id string, body JSON) (JSON, error) {
	return s.c.json(ctx, "POST", "/imports/"+url.PathEscape(id)+"/save-to-group", nil, body)
}

// JobsService reads background job progress.
type JobsService struct{ c *Client }

// Get returns a job's status and progress.
func (s *JobsService) Get(ctx context.Context, id int) (JSON, error) {
	return s.c.json(ctx, "GET", "/jobs/"+idp(id), nil, nil)
}

// ContactsService is the contact book.
type ContactsService struct{ c *Client }

// List searches contacts (filters: search, group_id, opted_out, page, limit).
func (s *ContactsService) List(ctx context.Context, f ListQuery) (JSON, error) {
	return s.c.json(ctx, "GET", "/contacts", query(f), nil)
}

// Get returns one contact by E.164 phone.
func (s *ContactsService) Get(ctx context.Context, phone string) (JSON, error) {
	return s.c.json(ctx, "GET", "/contacts/"+url.PathEscape(phone), nil, nil)
}

// Upsert adds or updates a contact in groups (body: phone, group_ids, name, fields).
func (s *ContactsService) Upsert(ctx context.Context, body JSON) (JSON, error) {
	return s.c.json(ctx, "POST", "/contacts", nil, body)
}

// Update updates a contact everywhere it appears (fields are merged).
func (s *ContactsService) Update(ctx context.Context, phone string, body JSON) (JSON, error) {
	return s.c.json(ctx, "PATCH", "/contacts/"+url.PathEscape(phone), nil, body)
}

// Delete removes a contact from every group.
func (s *ContactsService) Delete(ctx context.Context, phone string) error {
	return s.c.do(ctx, "DELETE", "/contacts/"+url.PathEscape(phone), nil, nil, nil)
}

// GroupsService manages contact groups.
type GroupsService struct{ c *Client }

// List lists groups.
func (s *GroupsService) List(ctx context.Context, q string) (JSON, error) {
	return s.c.json(ctx, "GET", "/groups", query(map[string]string{"q": q}), nil)
}

// Create creates an empty group.
func (s *GroupsService) Create(ctx context.Context, name string, tags []string) (JSON, error) {
	return s.c.json(ctx, "POST", "/groups", nil, JSON{"name": name, "tags": tags})
}

// Get fetches a group.
func (s *GroupsService) Get(ctx context.Context, id int) (JSON, error) {
	return s.c.json(ctx, "GET", "/groups/"+idp(id), nil, nil)
}

// Update renames a group or changes its tags.
func (s *GroupsService) Update(ctx context.Context, id int, body JSON) (JSON, error) {
	return s.c.json(ctx, "PATCH", "/groups/"+idp(id), nil, body)
}

// Delete deletes a group and its rows.
func (s *GroupsService) Delete(ctx context.Context, id int) (JSON, error) {
	return s.c.json(ctx, "DELETE", "/groups/"+idp(id), nil, nil)
}

// Contacts lists a group's contacts (filters: search, page, limit).
func (s *GroupsService) Contacts(ctx context.Context, id int, f ListQuery) (JSON, error) {
	return s.c.json(ctx, "GET", "/groups/"+idp(id)+"/contacts", query(f), nil)
}

// GroupContact is one contact to add.
type GroupContact struct {
	Phone  string            `json:"phone"`
	Name   string            `json:"name,omitempty"`
	Fields map[string]string `json:"fields,omitempty"`
}

// AddContacts adds up to 10,000 contacts.
func (s *GroupsService) AddContacts(ctx context.Context, id int, contacts []GroupContact, defaultCountry string) (JSON, error) {
	return s.c.json(ctx, "POST", "/groups/"+idp(id)+"/contacts", nil, omit(JSON{"contacts": contacts, "default_country": defaultCountry}))
}

// RemoveContact removes one number from a group.
func (s *GroupsService) RemoveContact(ctx context.Context, id int, phone string) (JSON, error) {
	return s.c.json(ctx, "DELETE", fmt.Sprintf("/groups/%d/contacts/%s", id, url.PathEscape(phone)), nil, nil)
}

// TemplatesService manages message templates.
type TemplatesService struct{ c *Client }

// List lists templates (filters: category, message_type, q).
func (s *TemplatesService) List(ctx context.Context, f ListQuery) (JSON, error) {
	return s.c.json(ctx, "GET", "/templates", query(f), nil)
}

// Create saves a template (body: name, text, category, message_type, description, visibility, sender_id).
func (s *TemplatesService) Create(ctx context.Context, body JSON) (JSON, error) {
	return s.c.json(ctx, "POST", "/templates", nil, body)
}

// Get fetches a template.
func (s *TemplatesService) Get(ctx context.Context, id int) (JSON, error) {
	return s.c.json(ctx, "GET", "/templates/"+idp(id), nil, nil)
}

// Update partially updates a template.
func (s *TemplatesService) Update(ctx context.Context, id int, body JSON) (JSON, error) {
	return s.c.json(ctx, "PATCH", "/templates/"+idp(id), nil, body)
}

// Delete deletes a template.
func (s *TemplatesService) Delete(ctx context.Context, id int) error {
	return s.c.do(ctx, "DELETE", "/templates/"+idp(id), nil, nil, nil)
}

// StarterPack lists the ready-made templates.
func (s *TemplatesService) StarterPack(ctx context.Context) (JSON, error) {
	return s.c.json(ctx, "GET", "/templates/starter-pack", nil, nil)
}

// UseStarter copies a starter template (e.g. "payment-reminder") into your library.
func (s *TemplatesService) UseStarter(ctx context.Context, key string) (JSON, error) {
	return s.c.json(ctx, "POST", "/templates/starter-pack/"+url.PathEscape(key), nil, nil)
}

// Render renders a template with sample values (no send).
func (s *TemplatesService) Render(ctx context.Context, text string, vars map[string]string) (JSON, error) {
	return s.c.json(ctx, "POST", "/templates/render", nil, JSON{"text": text, "vars": vars})
}

func omit(m JSON) JSON {
	for k, v := range m {
		if s, ok := v.(string); ok && s == "" {
			delete(m, k)
		}
	}
	return m
}
