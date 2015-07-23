// Default implementation of Authenticator interface.
// You can swap this with a stub implementation in tests like so:
//
//		func stubAuthenticatorFactory() endpoints.Authenticator {
//			// Create a stub which implements (or probably fakes)
//			// endpoints.Authenticator
//		}
//
//		func TestSomething(t *testing.T) {
//			origFactory = endpoints.AuthenticatorFactory
//			endpoints.AuthenticatorFactory = stubAuthenticatorFactory
//			defer func() {
//				endpoints.AuthenticatorFactory = origFactory
//			}
//			// Do some testing here.
//			// Any call in the code that (indirectly) does
//			// "endpoints.NewContext(r)" will actually invoke
//			// stubAuthenticatorFactory() now.
//		}

package endpoints

import (
	"sync"

	"golang.org/x/net/context"
	"google.golang.org/appengine/internal"
	pb "google.golang.org/appengine/internal/user"
	"google.golang.org/appengine/user"
)

type cachingAuthenticator struct {
	// map keys are scopes
	oauthResponseCache map[string]*pb.GetOAuthUserResponse
	// mutex for oauthResponseCache
	sync.Mutex
}

// populateOAuthResponse updates (overwrites) OAuth user data associated
// with this request and the given scope.  It should only be called
// while the mutex is held.
func (ca *cachingAuthenticator) populateOAuthResponse(c context.Context, scope string) error {
	// Only one scope should be cached at once, so we just destroy the cache
	ca.oauthResponseCache = map[string]*pb.GetOAuthUserResponse{}

	req := &pb.GetOAuthUserRequest{Scope: &scope}
	res := &pb.GetOAuthUserResponse{}

	err := internal.Call(c, "user", "GetOAuthUser", req, res)
	if err != nil {
		return err
	}

	ca.oauthResponseCache[scope] = res
	return nil
}

func (ca *cachingAuthenticator) oauthResponse(c context.Context, scope string) (*pb.GetOAuthUserResponse, error) {
	ca.Lock()
	defer ca.Unlock()

	res, ok := ca.oauthResponseCache[scope]
	if !ok {
		if err := ca.populateOAuthResponse(c, scope); err != nil {
			return nil, err
		}
		res = ca.oauthResponseCache[scope]
	}
	return res, nil
}

// CurrentOAuthClientID returns a clientID associated with the scope.
func (ca *cachingAuthenticator) CurrentOAuthClientID(c context.Context, scope string) (string, error) {
	res, err := ca.oauthResponse(c, scope)
	if err != nil {
		return "", err
	}
	return res.GetClientId(), nil
}

// CurrentOAuthUser returns a user of this request for the given scope.
// It caches OAuth info at the first call for future invocations.
//
// Returns an error if data for this scope is not available.
func (ca *cachingAuthenticator) CurrentOAuthUser(c context.Context, scope string) (*user.User, error) {
	res, err := ca.oauthResponse(c, scope)
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

// Default implentation of endpoints.AuthenticatorFactory.
func cachingAuthenticatorFactory() Authenticator {
	// TODO(dhermes): Check whether the prod behaviour is identical to dev.
	// On dev appengine.NewContext() panics on error so, if it is identical
	// then there's nothing else to do here.
	// (was: Fail if ctx is nil.)
	return &cachingAuthenticator{
		oauthResponseCache: make(map[string]*pb.GetOAuthUserResponse),
	}
}
