// This implementation of Context interface uses tokeninfo API to validate
// bearer token.
// 
// It is intended to use on dev server but will work on production too, although
// it probably won't make as much sense.

package endpoints

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"appengine"
	"appengine/memcache"
	"appengine/urlfetch"
	"appengine/user"
)

const (
	tokeninfoEndpointUrl = "https://www.googleapis.com/oauth2/v2/tokeninfo"
	tokeninfoContextNS   = "__tokeninfo"
)

type tokeninfo struct {
	IssuedTo      string `json:"issued_to"`
	Audience      string `json:"audience"`
	UserId        string `json:"user_id"`
	Scope         string `json:"scope"`
	ExpiresIn     int    `json:"expires_in"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	AccessType    string `json:"access_type"`
}

// fetchTokeninfo retrieves token info from tokeninfoEndpointUrl and
// caches successful results with expiration set to tokeinfo.ExpiresIn
// and key of the token.
// 
// It uses a separate namespace.
func fetchTokeninfo(c Context, token string) (*tokeninfo, error) {
	ns, err := appengine.Namespace(c, tokeninfoContextNS)
	if err != nil {
		return nil, err
	}

	ti := &tokeninfo{}
	_, err = memcache.JSON.Get(ns, token, ti)
	if err == nil {
		return ti, nil
	}

	client := urlfetch.Client(c)
	url := tokeninfoEndpointUrl + "?access_token=" + token
	c.Debugf("Fetching token info from %q", url)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Error from tokeninfo: %d", resp.StatusCode)
	}

	err = json.NewDecoder(resp.Body).Decode(ti)

	if err == nil && ti.ExpiresIn > 0 {
		item := &memcache.Item{
			Key:        token,
			Object:     ti,
			Expiration: time.Duration(ti.ExpiresIn) * time.Second,
		}
		memcache.JSON.Set(ns, item)
	}
	return ti, err
}

// getScopedTokeninfo validates fetched token by matching tokeinfo.Scope
// with scope arg.
func getScopedTokeninfo(c Context, scope string) (*tokeninfo, error) {
	token := getToken(c.HttpRequest())
	if token == "" {
		return nil, errors.New("No token found")
	}
	ti, err := fetchTokeninfo(c, token)
	if err != nil {
		return nil, err
	}
	for _, s := range strings.Split(ti.Scope, " ") {
		if s == scope {
			return ti, nil
		}
	}
	return nil, fmt.Errorf("No scope matches: expected one of %q, got %q",
		ti.Scope, scope)
}

// A context that uses tokeninfo API to validate bearer token
type tokeninfoContext struct {
	appengine.Context
	r *http.Request
}

func (c *tokeninfoContext) HttpRequest() *http.Request {
	return c.r
}

// CurrentOAuthClientID returns a clientId associated with the scope.
func (c *tokeninfoContext) CurrentOAuthClientID(scope string) (string, error) {
	ti, err := getScopedTokeninfo(c, scope)
	if err != nil {
		return "", err
	}
	return ti.IssuedTo, nil
}

// CurrentOAuthUser returns a user associated with the request in context.
func (c *tokeninfoContext) CurrentOAuthUser(scope string) (*user.User, error) {
	ti, err := getScopedTokeninfo(c, scope)
	if err != nil {
		return nil, err
	}
	return &user.User{
		Email: ti.Email,
		ID:    ti.UserId,
	}, nil
}

// tokeninfoContextFactory creates a new tokeninfoContext from r.
// To be used as auth.go/ContextFactory.
func tokeninfoContextFactory(r *http.Request) Context {
	ac := appengine.NewContext(r)
	return &tokeninfoContext{ac, r}
}
