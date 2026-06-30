package gworkspace

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
)

type fakeTokenStore struct {
	token string
	err   error
	saved map[string]string
}

func (f *fakeTokenStore) GetRefreshToken(_ context.Context, _ string) (string, error) {
	return f.token, f.err
}

func (f *fakeTokenStore) SaveRefreshToken(_ context.Context, owner, refreshToken string) error {
	if f.saved == nil {
		f.saved = map[string]string{}
	}
	f.saved[owner] = refreshToken
	return nil
}

// TokenSource must surface ErrNotConnected from the store so domain clients
// (Calendar, Gmail, Contacts) can propagate it to callers without touching
// the network.
func TestTokenSourceNotConnected(t *testing.T) {
	c := NewClient(&fakeTokenStore{err: ErrNotConnected}, &oauth2.Config{})
	_, err := c.TokenSource(context.Background(), "owner")
	if !errors.Is(err, ErrNotConnected) {
		t.Errorf("TokenSource err = %v, want ErrNotConnected", err)
	}
}
