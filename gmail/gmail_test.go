package gmail

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"golang.org/x/oauth2"
	gmailv1 "google.golang.org/api/gmail/v1"

	"go.naturallyfunny.dev/gworkspace"
)

type fakeConnector struct{ err error }

func (f *fakeConnector) TokenSourceFor(_ context.Context, _ string) (oauth2.TokenSource, error) {
	return nil, f.err
}

func TestNotConnectedPropagates(t *testing.T) {
	svc := New(&fakeConnector{err: gworkspace.ErrNotConnected})
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
	}{
		{"ReadMessages", func() error { _, err := svc.ReadMessages(ctx, "owner", MessageQuery{}); return err }},
		{"GetMessagesByLabel", func() error { _, err := svc.GetMessagesByLabel(ctx, "owner", "INBOX", LabelQuery{}); return err }},
		{"SendEmail", func() error { return svc.SendEmail(ctx, "owner", "to@example.com", "s", "b") }},
		{"GetLabels", func() error { _, err := svc.GetLabels(ctx, "owner"); return err }},
		{"CreateLabel", func() error { _, err := svc.CreateLabel(ctx, "owner", "name"); return err }},
		{"ApplyLabel", func() error { return svc.ApplyLabel(ctx, "owner", "msg", "lbl") }},
	}
	for _, tt := range tests {
		if err := tt.call(); !errors.Is(err, gworkspace.ErrNotConnected) {
			t.Errorf("%s err = %v, want ErrNotConnected", tt.name, err)
		}
	}
}

func TestMessageFrom(t *testing.T) {
	body := "Hello there"
	m := &gmailv1.Message{
		Id:           "msg1",
		ThreadId:     "thread1",
		Snippet:      "Hello...",
		LabelIds:     []string{"INBOX", "UNREAD"},
		InternalDate: time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC).UnixMilli(),
		Payload: &gmailv1.MessagePart{
			MimeType: "multipart/alternative",
			Headers: []*gmailv1.MessagePartHeader{
				{Name: "From", Value: "alice@example.com"},
				{Name: "To", Value: "bob@example.com"},
				{Name: "Subject", Value: "Greetings"},
			},
			Parts: []*gmailv1.MessagePart{
				{MimeType: "text/plain", Body: &gmailv1.MessagePartBody{Data: base64.URLEncoding.EncodeToString([]byte(body))}},
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
	got := labelFrom(&gmailv1.Label{Id: "Label_1", Name: "Work", Type: "user"})
	want := Label{ID: "Label_1", Name: "Work", Type: "user"}
	if got != want {
		t.Errorf("labelFrom() = %+v, want %+v", got, want)
	}
}

func TestDecodeBodyUnpadded(t *testing.T) {
	raw := "abc"
	encoded := base64.RawURLEncoding.EncodeToString([]byte(raw))
	if got := decodeBody(encoded); got != raw {
		t.Errorf("decodeBody() = %q, want %q", got, raw)
	}
}
