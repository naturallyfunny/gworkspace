// Package auth provides the OAuth client for Google Workspace.
// Build one Client and pass it to any combination of domain constructors:
//
//	a   := auth.New(store, cfg)
//	cal := gworkspace.NewCalendar(a)
//	gm  := gworkspace.NewGmail(a)
//	con := gworkspace.NewContacts(a)
//
// cfg.Scopes determines which Workspace APIs are accessible. Assemble the
// scope list from the gworkspace.*RequiredScopes variables, e.g.:
//
//	cfg.Scopes = append(gworkspace.CalendarRequiredScopes, gworkspace.GmailRequiredScopes...)
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/oauth2"

	"go.naturallyfunny.dev/gworkspace"
)

// Client holds an *oauth2.Config and a TokenStore; it builds a fresh token
// source per request from the owner's stored refresh token.
// Client implements gworkspace.Auth.
type Client struct {
	tokenStore TokenStore
	oauth2Cfg  *oauth2.Config
}
var _ gworkspace.Auth = (*Client)(nil)

// TokenStore persists Google Workspace OAuth refresh tokens keyed by owner.
// Implement this interface to provide your own storage backend.
// GetRefreshToken must return gworkspace.ErrNotConnected when no token exists
// for the owner. postgres.TokenStore in this module satisfies it.
type TokenStore interface {
	GetRefreshToken(ctx context.Context, owner string) (string, error)
	SaveRefreshToken(ctx context.Context, owner, refreshToken string) error
}

// New builds a Client. cfg must carry the Google endpoint and the combined
// scope list for all Workspace APIs the consumer will use.
func New(tokenStore TokenStore, cfg *oauth2.Config) *Client {
	return &Client{tokenStore: tokenStore, oauth2Cfg: cfg}
}

// Scopes returns the OAuth scopes this client was configured with.
// Domain constructors (NewCalendar, NewGmail, NewContacts) call this to verify
// the client carries the scopes they need before accepting it.
func (c *Client) Scopes() []string {
	return c.oauth2Cfg.Scopes
}

// AuthURL returns the Google consent URL the user must visit to grant access.
// state is handed to Google and returned verbatim on the callback; consumers
// use it to correlate the callback with a user and to guard against CSRF.
//
// Offline access plus prompt=consent are requested so Google returns a refresh
// token: without offline access there is no refresh token at all, and without
// forcing the consent screen Google omits it on re-authorization of an account
// that has already granted access.
func (c *Client) AuthURL(state string) string {
	return c.oauth2Cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
}

// ErrMissingScopes is returned by Exchange when Google did not grant all scopes
// that were requested in cfg.Scopes — usually because the user deselected some
// permissions on the consent screen. Trigger a new OAuth flow to re-request.
var ErrMissingScopes = errors.New("gworkspace/auth: google did not grant all requested scopes")

// Exchange completes the OAuth flow by trading the authorization code from the
// Google callback for tokens, returning the refresh token to persist via
// TokenStore.SaveRefreshToken. The code is single-use.
// Returns ErrMissingScopes if Google did not grant all scopes in cfg.Scopes.
func (c *Client) Exchange(ctx context.Context, code string) (string, error) {
	token, err := c.oauth2Cfg.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("exchange code: %w", err)
	}
	if token.RefreshToken == "" {
		return "", errors.New("gworkspace/auth: exchange returned no refresh token")
	}
	if len(c.oauth2Cfg.Scopes) > 0 {
		granted := strings.Fields(fmt.Sprintf("%s", token.Extra("scope")))
		if missing := missingScopes(c.oauth2Cfg.Scopes, granted); len(missing) > 0 {
			return "", fmt.Errorf("%w: missing %s", ErrMissingScopes, strings.Join(missing, ", "))
		}
	}
	return token.RefreshToken, nil
}

// Connect completes the OAuth flow for owner: it trades code for tokens via
// Exchange and persists the resulting refresh token.
func (c *Client) Connect(ctx context.Context, owner, code string) error {
	refreshToken, err := c.Exchange(ctx, code)
	if err != nil {
		return err
	}
	return c.tokenStore.SaveRefreshToken(ctx, owner, refreshToken)
}

// TokenSource resolves the owner's refresh token and returns an oauth2.TokenSource
// that refreshes access tokens on demand. Returns gworkspace.ErrNotConnected
// when the owner has not connected. TokenSource implements gworkspace.Auth.
func (c *Client) TokenSource(ctx context.Context, owner string) (oauth2.TokenSource, error) {
	refreshToken, err := c.tokenStore.GetRefreshToken(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("get token for owner %s: %w", owner, err)
	}
	return c.oauth2Cfg.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken}), nil
}

func missingScopes(required, granted []string) []string {
	grantedSet := make(map[string]bool, len(granted))
	for _, s := range granted {
		grantedSet[s] = true
	}
	var missing []string
	for _, s := range required {
		if !grantedSet[s] {
			missing = append(missing, s)
		}
	}
	return missing
}
