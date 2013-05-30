package endpoints

import (
	"net/http"
	"sync"

	"appengine"
	"appengine/user"
	"errors"

	pb "appengine_internal/user"
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
	ctxs   = make(map[*http.Request]*CachingContext)
)

// CachingContext represents the context of an in-flight API request.
// Also, it contains appengine.Context, so can be used in standard appengine/*
// packages.
type CachingContext struct {
	appengine.Context
	// map keys are scopes
	oauthResponseCache map[string]*pb.GetOAuthUserResponse
}

// NewContext returns a new context for an in-flight API (HTTP) request.
func NewContext(req *http.Request) *CachingContext {
	ctxsMu.Lock()
	defer ctxsMu.Unlock()
	cachingCtx, ok := ctxs[req]

	if !ok {
		// TODO(dhermes): Fail if ctx is nil.
		ctx := appengine.NewContext(req)
		cachingCtx = &CachingContext{ctx, map[string]*pb.GetOAuthUserResponse{}}
		ctxs[req] = cachingCtx
	}

	return cachingCtx
}

// PopulateOAuthResponse updates (overwrites) OAuth user data associated with
// this request and the given scope.
func (ctx *CachingContext) PopulateOAuthResponse(scope string) error {
	// Only one scope should be cached at once, so we just destroy the cache
	ctx.oauthResponseCache = map[string]*pb.GetOAuthUserResponse{}

	req := &pb.GetOAuthUserRequest{Scope: &scope}
	res := &pb.GetOAuthUserResponse{}

	err := ctx.Call("user", "GetOAuthUser", req, res, nil)
	if err != nil {
		return err
	}

	ctx.oauthResponseCache[scope] = res
	return nil
}

// CachedCurrentOAuthClientID returns a clientId associated with the scope.
func (ctx *CachingContext) CachedCurrentOAuthClientID(scope string) (*string, error) {
	res, ok := ctx.oauthResponseCache[scope]

	if !ok {
		err := ctx.PopulateOAuthResponse(scope)
		if err != nil {
			return nil, err
		}
		res = ctx.oauthResponseCache[scope]
	}

	result := res.GetClientId()
	return &result, nil
}

// CachedCurrentOAuthUser returns a user of this request for the given scope.
// It caches OAuth info at the first call for future invocations.
// 
// Returns an error if data for this scope is not available.
func (ctx *CachingContext) CachedCurrentOAuthUser(scope string) (*user.User, error) {
	res, ok := ctx.oauthResponseCache[scope]

	if !ok {
		err := ctx.PopulateOAuthResponse(scope)
		if err != nil {
			return nil, err
		}
		res = ctx.oauthResponseCache[scope]
	}

	return &user.User{
		Email:      *res.Email,
		AuthDomain: *res.AuthDomain,
		Admin:      res.GetIsAdmin(),
		ID:         *res.UserId,
	}, nil
}

// CurrentBearerTokenScope compares given scopes and clientIDs with those in ctx.
// Both scopes and clientIDs args must have at least one element.
// Returns a single scope (one of provided scopes) IIF two conditions are met:
//   - it is found in ctx
//   - client ID on that scope matches one of provided clientIDs from the args
func CurrentBearerTokenScope(ctx *CachingContext, scopes []string, clientIDs []string) (*string, error) {
	var clientID *string
	for _, scope := range scopes {
		clientID, _ = ctx.CachedCurrentOAuthClientID(scope)
		if clientID == nil {
			continue
		}

		for _, id := range clientIDs {
			if id == *clientID {
				return &scope, nil
			}
		}
		// If none of the client IDs matches, return nil
		return nil, errors.New("Mismatched Client ID")
	}
	return nil, errors.New("No valid scope.")
}

// CurrentBearerTokenUser returns a user associated with the request.
// Both scopes and clientIDs must have at least one element.
// Returns an error if a client did not make a valid request, or none of
// clientIDs are allowed to make requests, or user did not authorized any of
// the scopes.
func CurrentBearerTokenUser(req *http.Request, scopes []string, clientIDs []string) (*user.User, error) {
	ctx := NewContext(req)
	scope, err := CurrentBearerTokenScope(ctx, scopes, clientIDs)
	if scope == nil {
		return nil, err
	}

	return ctx.CachedCurrentOAuthUser(*scope)
}

// Currently a stub.
// See https://github.com/crhym3/go-endpoints/issues/2
func CurrentUserWithScope(c appengine.Context, scope string) (*user.User, error) {
	return user.CurrentOAuth(c, scope)
}
