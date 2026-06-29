package gworkspace

import (
	"encoding/base64"
	"testing"
	"time"

	gmail "google.golang.org/api/gmail/v1"
)

func TestMessageFrom(t *testing.T) {
	body := "Hello there"
	m := &gmail.Message{
		Id:           "msg1",
		ThreadId:     "thread1",
		Snippet:      "Hello...",
		LabelIds:     []string{"INBOX", "UNREAD"},
		InternalDate: time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC).UnixMilli(),
		Payload: &gmail.MessagePart{
			MimeType: "multipart/alternative",
			Headers: []*gmail.MessagePartHeader{
				{Name: "From", Value: "alice@example.com"},
				{Name: "To", Value: "bob@example.com"},
				{Name: "Subject", Value: "Greetings"},
			},
			Parts: []*gmail.MessagePart{
				{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: base64.URLEncoding.EncodeToString([]byte(body))}},
			},
		},
	}

	got := messageFrom(m)
	if got.ID != "msg1" || got.ThreadID != "thread1" {
		t.Errorf("ids = %q/%q", got.ID, got.ThreadID)
	}
	if got.From != "alice@example.com" || got.To != "bob@example.com" || got.Subject != "Greetings" {
		t.Errorf("headers = %q/%q/%q", got.From, got.To, got.Subject)
	}
	if got.Body != body {
		t.Errorf("Body = %q, want %q", got.Body, body)
	}
	if got.Date.UTC() != time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC) {
		t.Errorf("Date = %v", got.Date.UTC())
	}
	if len(got.Labels) != 2 {
		t.Errorf("Labels = %v", got.Labels)
	}
}

func TestLabelFrom(t *testing.T) {
	got := labelFrom(&gmail.Label{Id: "Label_1", Name: "Work", Type: "user"})
	want := Label{ID: "Label_1", Name: "Work", Type: "user"}
	if got != want {
		t.Errorf("labelFrom() = %+v, want %+v", got, want)
	}
}

func TestDecodeBodyUnpadded(t *testing.T) {
	raw := "abc" // base64url without padding
	encoded := base64.RawURLEncoding.EncodeToString([]byte(raw))
	if got := decodeBody(encoded); got != raw {
		t.Errorf("decodeBody() = %q, want %q", got, raw)
	}
}
