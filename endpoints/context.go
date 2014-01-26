// Default implementation of Context interface.
// You can swap this with a stub implementation in tests like so:
//
//		func stubContextFactory(r *http.Request) endpoints.Context {
//			// Create a stub which implements (or probably fakes)
//			// endpoints.Context
//		}
//
//		func TestSomething(t *testing.T) {
//			origFactory = endpoints.ContextFactory
//			endpoints.ContextFactory = stubContextFactory
//			defer func() {
//				endpoints.ContextFactory = origFactory
//			}
//			// Do some testing here.
//			// Any call in the code that (indirectly) does
//			// "endpoints.NewContext(r)" will actually invoke
//			// stubContextFactory(r) now.
//		}

package endpoints

import (
	"net/http"
	"sync"

	"appengine"
	"appengine/user"

	pb "appengine_internal/user"
)

type cachingContext struct {
	appengine.Context
	r *http.Request
	// map keys are scopes
	oauthResponseCache map[string]*pb.GetOAuthUserResponse
	// mutex for oauthResponseCache
	sync.Mutex
}

// populateOAuthResponse updates (overwrites) OAuth user data associated with
// this request and the given scope.
func populateOAuthResponse(c *cachingContext, scope string) error {
	// Only one scope should be cached at once, so we just destroy the cache
	c.oauthResponseCache = map[string]*pb.GetOAuthUserResponse{}

	req := &pb.GetOAuthUserRequest{Scope: &scope}
	res := &pb.GetOAuthUserResponse{}

	err := c.Call("user", "GetOAuthUser", req, res, nil)
	if err != nil {
		return err
	}

	c.oauthResponseCache[scope] = res
	return nil
}

func getOAuthResponse(c *cachingContext, scope string) (*pb.GetOAuthUserResponse, error) {
	res, ok := c.oauthResponseCache[scope]

	if !ok {
		c.Lock()
		defer c.Unlock()
		if err := populateOAuthResponse(c, scope); err != nil {
			return nil, err
		}
		res = c.oauthResponseCache[scope]
	}

	return res, nil
}

// HttpRequest returns the request associated with this context.
func (c *cachingContext) HttpRequest() *http.Request {
	return c.r
}

// CurrentOAuthClientID returns a clientId associated with the scope.
func (c *cachingContext) CurrentOAuthClientID(scope string) (string, error) {
	res, err := getOAuthResponse(c, scope)
	if err != nil {
		return "", err
	}
	return res.GetClientId(), nil
}

// CurrentOAuthUser returns a user of this request for the given scope.
// It caches OAuth info at the first call for future invocations.
//
// Returns an error if data for this scope is not available.
func (c *cachingContext) CurrentOAuthUser(scope string) (*user.User, error) {
	res, err := getOAuthResponse(c, scope)
	if err != nil {
		return nil, err
	}

	return &user.User{
		Email:      *res.Email,
		AuthDomain: *res.AuthDomain,
		Admin:      res.GetIsAdmin(),
		ID:         *res.UserId,
	}, nil
}

// Default implentation of endpoints.ContextFactory.
func cachingContextFactory(r *http.Request) Context {
	// TODO(dhermes): Check whether the prod behaviour is identical to dev.
	// On dev appengine.NewContext() panics on error so, if it is identical
	// then there's nothing else to do here.
	// (was: Fail if ctx is nil.)
	ac := appengine.NewContext(r)
	return &cachingContext{ac, r, map[string]*pb.GetOAuthUserResponse{}, sync.Mutex{}}
}
