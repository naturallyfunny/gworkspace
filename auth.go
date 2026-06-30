// Package gworkspace is a reusable client for a user's Google Workspace:
// callers reach Calendar, Gmail, and Contacts by their own opaque owner string.
//
// One OAuth connect per user covers the whole Workspace — Calendar, Gmail, and
// Contacts share a single refresh token. The consumer assembles the scope list
// from whichever *RequiredScopes vars match the capabilities they use:
//
//	cfg := &oauth2.Config{
//	    Scopes: append(gworkspace.CalendarRequiredScopes, gworkspace.GmailRequiredScopes...),
//	    ...
//	}
//	a   := auth.New(store, cfg)
//	cal, err := gworkspace.NewCalendar(a)
//	gm,  err := gworkspace.NewGmail(a)
package gworkspace

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/oauth2"
)

// Auth is the narrow interface that domain clients need to obtain per-owner
// OAuth token sources and to verify that the required scopes are present.
// *auth.Client implements it. Consumers can also satisfy it in tests without
// a real token store.
type Auth interface {
	TokenSource(ctx context.Context, owner string) (oauth2.TokenSource, error)
	Scopes() []string
}

// ErrNotConnected is returned when the owner has not completed the OAuth
// connect flow and therefore has no stored refresh token. Route the user
// through auth.Client.AuthURL / auth.Client.Connect to resolve it.
var ErrNotConnected = errors.New("gworkspace: user not connected")

// checkScopes returns an error if have does not contain every scope in need.
// Used by domain constructors to fail fast when the Auth was configured without
// the scopes that capability requires.
func checkScopes(have, need []string) error {
	haveSet := make(map[string]bool, len(have))
	for _, s := range have {
		haveSet[s] = true
	}
	var missing []string
	for _, s := range need {
		if !haveSet[s] {
			missing = append(missing, s)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("gworkspace: auth missing required scopes: %s", strings.Join(missing, ", "))
	}
	return nil
}
