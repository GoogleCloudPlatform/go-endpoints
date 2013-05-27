package endpoints

import (
	"appengine/user"
	"errors"
)

const (
	ClockSkewSecs        = 300
	MaxTokenLifetimeSecs = 86400
	DefaultCertUri       = ("https://www.googleapis.com/service_accounts/v1/metadata/raw/" +
		"federated-signon@system.gserviceaccount.com")
	EmailScope   = "https://www.googleapis.com/auth/userinfo.email"
	TokeninfoUrl = "https://www.googleapis.com/oauth2/v1/tokeninfo"
)

// Currently a stub.
// See https://github.com/crhym3/go-endpoints/issues/2
func CurrentUserWithScope(scope string) (*user.User, error) {
	return user.CurrentOAuth(c, scope)
}

// Should return user based on a last scope.
// Currently always returns a not-implemented error
func CurrentUser() (*user.User, error) {
	return nil, errors.New("Not implemented")
}
