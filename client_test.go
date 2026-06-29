package gworkspace

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
)

// fakeStore is a TokenStore whose behavior is controlled per test.
type fakeStore struct {
	token string
	err   error
	saved map[string]string
}

func (f *fakeStore) GetRefreshToken(ctx context.Context, owner string) (string, error) {
	return f.token, f.err
}

func (f *fakeStore) SaveRefreshToken(ctx context.Context, owner, refreshToken string) error {
	if f.saved == nil {
		f.saved = map[string]string{}
	}
	f.saved[owner] = refreshToken
	return nil
}

// When the store reports the user is not connected, every feature call must
// surface ErrNotConnected before touching the network, so consumers can route
// into the login flow. All methods resolve the token through the same
// tokenSourceFor path, so this covers every public capability.
func TestNotConnectedPropagates(t *testing.T) {
	c := New(&fakeStore{err: ErrNotConnected}, &oauth2.Config{})
	ctx := context.Background()

	tests := []struct {
		name string
		call func() error
	}{
		{"GetEvents", func() error { _, err := c.GetEvents(ctx, "owner", EventQuery{}); return err }},
		{"AddEvent", func() error { _, err := c.AddEvent(ctx, "owner", EventInput{}); return err }},
		{"ReadMessages", func() error { _, err := c.ReadMessages(ctx, "owner", ""); return err }},
		{"GetMessagesByLabel", func() error { _, err := c.GetMessagesByLabel(ctx, "owner", "INBOX"); return err }},
		{"SendEmail", func() error { return c.SendEmail(ctx, "owner", "to@example.com", "s", "b") }},
		{"GetLabels", func() error { _, err := c.GetLabels(ctx, "owner"); return err }},
		{"CreateLabel", func() error { _, err := c.CreateLabel(ctx, "owner", "name"); return err }},
		{"ApplyLabel", func() error { return c.ApplyLabel(ctx, "owner", "msg", "lbl") }},
		{"GetContacts", func() error { _, err := c.GetContacts(ctx, "owner"); return err }},
		{"AddContact", func() error { _, err := c.AddContact(ctx, "owner", ContactInput{}); return err }},
	}
	for _, tt := range tests {
		if err := tt.call(); !errors.Is(err, ErrNotConnected) {
			t.Errorf("%s err = %v, want ErrNotConnected", tt.name, err)
		}
	}
}

func TestSentinelFor(t *testing.T) {
	if got := sentinelFor(&googleapi.Error{Code: 429}); !errors.Is(got, ErrRateLimited) {
		t.Errorf("sentinelFor(429) = %v, want ErrRateLimited", got)
	}
	if got := sentinelFor(&googleapi.Error{Code: 404}); got != nil {
		t.Errorf("sentinelFor(404) = %v, want nil", got)
	}
	if got := sentinelFor(errors.New("boom")); got != nil {
		t.Errorf("sentinelFor(non-api) = %v, want nil", got)
	}
}

func TestWrapErrorNil(t *testing.T) {
	if err := wrapError("op", nil); err != nil {
		t.Errorf("wrapError(nil) = %v, want nil", err)
	}
}
