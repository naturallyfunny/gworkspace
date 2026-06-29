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

// TokenSourceFor must surface ErrNotConnected from the store so product
// subpackages (calendar, gmail, contact) can propagate it to callers without
// touching the network.
func TestTokenSourceForNotConnected(t *testing.T) {
	c := New(&fakeStore{err: ErrNotConnected}, &oauth2.Config{})
	_, err := c.TokenSourceFor(context.Background(), "owner")
	if !errors.Is(err, ErrNotConnected) {
		t.Errorf("TokenSourceFor err = %v, want ErrNotConnected", err)
	}
}

func TestWrapError(t *testing.T) {
	if err := WrapError("op", &googleapi.Error{Code: 429}); !errors.Is(err, ErrRateLimited) {
		t.Errorf("WrapError(429) does not wrap ErrRateLimited: %v", err)
	}
	if err := WrapError("op", &googleapi.Error{Code: 404}); errors.Is(err, ErrRateLimited) {
		t.Errorf("WrapError(404) incorrectly wraps ErrRateLimited")
	}
	if err := WrapError("op", nil); err != nil {
		t.Errorf("WrapError(nil) = %v, want nil", err)
	}
}
