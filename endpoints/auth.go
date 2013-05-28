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
	DefaultCertUri       = ("https://www.googleapis.com/service_accounts/v1/metadata/raw/" +
		"federated-signon@system.gserviceaccount.com")
	EmailScope   = "https://www.googleapis.com/auth/userinfo.email"
	TokeninfoUrl = "https://www.googleapis.com/oauth2/v1/tokeninfo"
	ApiExplorerClientId  = "292824132082.apps.googleusercontent.com"
)

var (
	ctxsMu sync.Mutex
	ctxs   = make(map[*http.Request]*CachingContext)
)

type CachingContext struct {
	appengine.Context
	oauthResponseCache map[string]*pb.GetOAuthUserResponse
}

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

func (ctx *CachingContext) PopulateOAuthResponse(scope string) error {
	// Only one scope should be cached at once, so we just destroy the cache
	ctx.oauthResponseCache = map[string]*pb.GetOAuthUserResponse{}

	req := &pb.GetOAuthUserRequest{}
	req.Scope = &scope
	res := &pb.GetOAuthUserResponse{}

	err := ctx.Call("user", "GetOAuthUser", req, res, nil)
	if err != nil {
		return err
	}

	ctx.oauthResponseCache[scope] = res
	return nil
}

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

func CurrentBearerTokenScope(ctx *CachingContext, scopes []string, clientIDs []string) (*string, error) {
	var clientID *string
	for i := 0; i < len(scopes); i++ {
		clientID, _ = ctx.CachedCurrentOAuthClientID(scopes[i])
		if clientID == nil {
		   continue;
		}

		for j := 0; j < len(clientIDs); j++ {
		    if clientIDs[j] == *clientID {
		        return &scopes[i], nil
		    }
		}
		// If none of the client IDs matches, return nil
		return nil, errors.New("Mismatched Client ID")
	}
	return nil, errors.New("No valid scope.")
}

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

// Should return user based on a last scope.
// Currently always returns a not-implemented error
func CurrentUser() (*user.User, error) {
	return nil, errors.New("Not implemented")
}
