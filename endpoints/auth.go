package endpoints

import (
	"errors"
	"net/http"
	"sync"

	"appengine"
	"appengine/user"
)

const (
	ClockSkewSecs        = 300
	MaxTokenLifetimeSecs = 86400
	DefaultCertUri       = ("https://www.googleapis.com/service_accounts/" +
		"v1/metadata/raw/federated-signon@system.gserviceaccount.com")
	EmailScope          = "https://www.googleapis.com/auth/userinfo.email"
	TokeninfoUrl        = "https://www.googleapis.com/oauth2/v1/tokeninfo"
	ApiExplorerClientId = "292824132082.apps.googleusercontent.com"
)

var (
	ctxsMu sync.Mutex
	ctxs   = make(map[*http.Request]Context)

	// ContextFactory takes an in-flight HTTP request and creates a new
	// context. 
	// 
	// It is a variable on purpose. You can set it to a stub implementation
	// in tests.
	ContextFactory func(*http.Request) Context
)

// Context represents the context of an in-flight API request.
// It embeds appengine.Context so you can use with any other appengine/*
// package methods.
type Context interface {
	appengine.Context

	// CurrentOAuthClientID returns a clientId associated with the scope.
	CurrentOAuthClientID(scope string) (string, error)

	// CurrentOAuthUser returns a user of this request for the given scope.
	// It caches OAuth info at the first call for future invocations.
	// 
	// Returns an error if data for this scope is not available.
	CurrentOAuthUser(scope string) (*user.User, error)
}

// NewContext returns a new context for an in-flight API (HTTP) request.
func NewContext(req *http.Request) Context {
	ctxsMu.Lock()
	defer ctxsMu.Unlock()
	c, ok := ctxs[req]

	if !ok {
		c = ContextFactory(req)
		ctxs[req] = c
	}

	return c
}

// CurrentBearerTokenScope compares given scopes and clientIDs with those in c.
// Both scopes and clientIDs args must have at least one element.
// Returns a single scope (one of provided scopes) IIF two conditions are met:
//   - it is found in Context c
//   - client ID on that scope matches one of clientIDs in the args
func CurrentBearerTokenScope(c Context, scopes []string, clientIDs []string) (string, error) {
	for _, scope := range scopes {
		clientID, err := c.CurrentOAuthClientID(scope)
		if err != nil {
			continue
		}

		for _, id := range clientIDs {
			if id == clientID {
				return scope, nil
			}
		}
		// If none of the client IDs matches, return nil
		return "", errors.New("Mismatched Client ID")
	}
	return "", errors.New("No valid scope")
}

// CurrentBearerTokenUser returns a user associated with the request.
// Both scopes and clientIDs must have at least one element.
// Returns an error if a client did not make a valid request, or none of
// clientIDs are allowed to make requests, or user did not authorized any of
// the scopes.
func CurrentBearerTokenUser(c Context, scopes []string, clientIDs []string) (*user.User, error) {
	scope, err := CurrentBearerTokenScope(c, scopes, clientIDs)
	if err != nil {
		return nil, err
	}

	return c.CurrentOAuthUser(scope)
}
