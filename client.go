// Package gworkspace is a reusable client for a user's Google Workspace: callers
// reach Calendar, Gmail, and Contacts by their own opaque owner ID instead of a
// Google account identifier.
//
// One OAuth connect per user covers the whole Workspace: Calendar, Gmail, and
// Contacts share a single refresh token granted against the combined
// RequiredScopes. The library is credential-free — it never builds an
// *oauth2.Config or a database pool itself; both arrive via dependency injection
// through New. Token persistence is delegated to a consumer-supplied TokenStore
// (a ready-made PostgreSQL one lives in the postgres subpackage).
//
// Product services (Calendar, Gmail, Contacts) live in subpackages and each
// accept a Connector — typically the *Client built here:
//
//	gwClient := gworkspace.New(store, cfg)
//	calSvc   := calendar.New(gwClient)
//	gmailSvc := gmail.New(gwClient)
//
// The package has zero knowledge of any application: an owner ID is whatever
// opaque string the consumer uses to identify a human, and nothing here depends
// on the meaning behind it.
package gworkspace

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/oauth2"
	calendar "google.golang.org/api/calendar/v3"
	gmail "google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
	people "google.golang.org/api/people/v1"
)

// ErrNotConnected indicates the user has no stored Workspace credentials, i.e.
// they have not completed the OAuth connect flow. TokenStore implementations
// must return this from GetRefreshToken when no token exists, so consumers can
// route the user into the login flow.
var ErrNotConnected = errors.New("gworkspace: user not connected")

// ErrRateLimited means Google is throttling the application (HTTP 429). It can
// surface from any call, so it lives here rather than with a single feature.
// Back off and retry. Match it with errors.Is.
var ErrRateLimited = errors.New("gworkspace: rate limited")

// RequiredScopes are the OAuth scopes every capability in this library needs.
// Pass them to the *oauth2.Config handed to New so the consent screen requests
// the right permissions, e.g.:
//
//	cfg := &oauth2.Config{
//	    ClientID:     id,
//	    ClientSecret: secret,
//	    RedirectURL:  redirectURL,
//	    Endpoint:     google.Endpoint,
//	    Scopes:       gworkspace.RequiredScopes,
//	}
//
// Mapping of scope to capability:
//   - calendar:     GetEvents, AddEvent
//   - gmail.modify: ReadMessages, GetLabels, ApplyLabel, CreateLabel, GetMessagesByLabel
//   - gmail.send:   SendEmail
//   - contacts:     GetContacts, AddContact
var RequiredScopes = []string{
	calendar.CalendarScope,
	gmail.GmailModifyScope,
	gmail.GmailSendScope,
	people.ContactsScope,
}

// Connector is the narrow credential interface that product subpackages need.
// *Client implements it. Consumers can also satisfy it in tests without a real
// token store.
type Connector interface {
	TokenSourceFor(ctx context.Context, owner string) (oauth2.TokenSource, error)
}

// TokenStore persists Google Workspace OAuth refresh tokens on behalf of a user.
// Implement this interface to provide your own storage backend.
// GetRefreshToken must return ErrNotConnected when no token exists for owner.
type TokenStore interface {
	GetRefreshToken(ctx context.Context, owner string) (string, error)
	// SaveRefreshToken stores (or replaces) the refresh token for owner.
	// It is called after a successful OAuth exchange.
	SaveRefreshToken(ctx context.Context, owner, refreshToken string) error
}

// Client provides access to a user's Google Workspace (Calendar, Gmail,
// Contacts) on behalf of an owner. It holds one *oauth2.Config (Google) and a
// TokenStore; it builds a fresh Google service per request from the owner's
// stored refresh token.
type Client struct {
	tokenStore TokenStore
	oauth2Cfg  *oauth2.Config
}

// New builds a Client over a TokenStore and a Google *oauth2.Config. The config
// must carry the Google endpoint and RequiredScopes; the library never
// constructs credentials itself. postgres.Store satisfies TokenStore, but any
// implementation works.
func New(tokenStore TokenStore, cfg *oauth2.Config) *Client {
	return &Client{tokenStore: tokenStore, oauth2Cfg: cfg}
}

// AuthURL returns the Google consent URL the user must visit to grant access.
// state is handed to Google and returned verbatim on the callback; consumers use
// it to correlate the callback with a user and to guard against CSRF. The
// redirect URI and scopes are taken from the *oauth2.Config supplied to New.
//
// Offline access plus prompt=consent are requested so Google returns a refresh
// token: without offline access there is no refresh token at all, and without
// forcing the consent screen Google omits it on re-authorization of an account
// that has already granted access. prompt=consent is Google's current parameter;
// the older approval_prompt=force is deprecated.
func (c *Client) AuthURL(state string) string {
	return c.oauth2Cfg.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
}

// Exchange completes the OAuth flow by trading the authorization code from the
// Google callback for tokens, returning the refresh token to persist via
// TokenStore.SaveRefreshToken. The code is single-use. It errors if Google
// returns no refresh token (typically because offline access was not requested,
// or the account already granted access without forcing consent — see AuthURL).
func (c *Client) Exchange(ctx context.Context, code string) (string, error) {
	token, err := c.oauth2Cfg.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("exchange code: %w", err)
	}
	if token.RefreshToken == "" {
		return "", errors.New("gworkspace: exchange returned no refresh token")
	}
	return token.RefreshToken, nil
}

// Connect completes the OAuth flow for owner: it trades code for tokens via
// Exchange and persists the resulting refresh token through TokenStore. From the
// caller's point of view this is atomic — no token is handed back to manage. Use
// it instead of Exchange + SaveRefreshToken when the consumer has no need for the
// raw token.
func (c *Client) Connect(ctx context.Context, owner, code string) error {
	refreshToken, err := c.Exchange(ctx, code)
	if err != nil {
		return err
	}
	return c.tokenStore.SaveRefreshToken(ctx, owner, refreshToken)
}

// TokenSourceFor resolves the owner's refresh token and returns an
// oauth2.TokenSource that refreshes access tokens on demand. It returns
// ErrNotConnected (from the store) when the owner has not connected.
// TokenSourceFor implements Connector.
func (c *Client) TokenSourceFor(ctx context.Context, owner string) (oauth2.TokenSource, error) {
	refreshToken, err := c.tokenStore.GetRefreshToken(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("get token for owner %s: %w", owner, err)
	}
	return c.oauth2Cfg.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken}), nil
}

// WrapError annotates err with the operation name and, when it carries a
// recognizable Google API error, joins one of the package sentinels so callers
// can branch with errors.Is. Returns nil when err is nil. Product subpackages
// route their API errors through here.
func WrapError(op string, err error) error {
	if err == nil {
		return nil
	}
	if sentinel := sentinelFor(err); sentinel != nil {
		return fmt.Errorf("%s: %w: %w", op, sentinel, err)
	}
	return fmt.Errorf("%s: %w", op, err)
}

// sentinelFor maps a Google API error to a package sentinel, or returns nil when
// none applies.
func sentinelFor(err error) error {
	var apiErr *googleapi.Error
	if !errors.As(err, &apiErr) {
		return nil
	}
	if apiErr.Code == 429 {
		return ErrRateLimited
	}
	return nil
}
