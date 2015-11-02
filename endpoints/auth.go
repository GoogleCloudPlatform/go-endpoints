package endpoints

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/memcache"
	"google.golang.org/appengine/user"
)

const (
	// DefaultCertURI is Google's public URL which points to JWT certs.
	DefaultCertURI = ("https://www.googleapis.com/service_accounts/" +
		"v1/metadata/raw/federated-signon@system.gserviceaccount.com")
	// EmailScope is Google's OAuth 2.0 email scope
	EmailScope = "https://www.googleapis.com/auth/userinfo.email"
	// TokeninfoURL is Google's OAuth 2.0 access token verification URL
	TokeninfoURL = "https://www.googleapis.com/oauth2/v1/tokeninfo"
	// APIExplorerClientID is the client ID of API explorer.
	APIExplorerClientID = "292824132082.apps.googleusercontent.com"
)

var (
	allowedAuthSchemesUpper = [2]string{"OAUTH", "BEARER"}
	certNamespace           = "__verify_jwt"
	clockSkewSecs           = int64(300)   // 5 minutes in seconds
	maxTokenLifetimeSecs    = int64(86400) // 1 day in seconds
	maxAgePattern           = regexp.MustCompile(`\s*max-age\s*=\s*(\d+)\s*`)

	// This is a variable on purpose: can be stubbed with a different (fake)
	// implementation during tests.
	//
	// endpoints package code should always call jwtParser()
	// instead of directly invoking verifySignedJWT().
	jwtParser = verifySignedJWT

	// currentUTC returns current time in UTC.
	// This is a variable on purpose to be able to stub during testing.
	currentUTC = func() time.Time {
		return time.Now().UTC()
	}

	// AuthenticatorFactory creates a new Authenticator.
	//
	// It is a variable on purpose. You can set it to a stub implementation
	// in tests.
	AuthenticatorFactory func() Authenticator
)

// An Authenticator can identify the current user.
type Authenticator interface {
	// CurrentOAuthClientID returns a clientID associated with the scope.
	CurrentOAuthClientID(ctx context.Context, scope string) (string, error)

	// CurrentOAuthUser returns a user of this request for the given scope.
	// It caches OAuth info at the first call for future invocations.
	//
	// Returns an error if data for this scope is not available.
	CurrentOAuthUser(ctx context.Context, scope string) (*user.User, error)
}

// contextKey is used to store values on a context.
type contextKey int

// Context value keys.
const (
	invalidKey contextKey = iota
	requestKey
	authenticatorKey
)

// HTTPRequest returns the request associated with a context.
func HTTPRequest(c context.Context) *http.Request {
	r, _ := c.Value(requestKey).(*http.Request)
	return r
}

// authenticator returns the Authenticator associated with a
// context, or nil if there is not one.
func authenticator(c context.Context) Authenticator {
	a, _ := c.Value(authenticatorKey).(Authenticator)
	return a
}

// Errors for incorrect contexts.
var (
	errNoAuthenticator = errors.New("context has no authenticator (use endpoints.NewContext to create a context)")
	errNoRequest       = errors.New("no request for context (use endpoints.NewContext to create a context)")
)

// NewContext returns a new context for an in-flight API (HTTP) request.
func NewContext(r *http.Request) context.Context {
	c := appengine.NewContext(r)
	c = context.WithValue(c, requestKey, r)
	c = context.WithValue(c, authenticatorKey, AuthenticatorFactory())
	return c
}

// parseToken looks for Authorization header and returns a token.
//
// Returns empty string if req does not contain authorization header
// or its value is not prefixed with allowedAuthSchemesUpper.
func parseToken(r *http.Request) string {
	// TODO(dhermes): Allow a struct with access_token and bearer_token
	//                fields here as well.
	pieces := strings.Fields(r.Header.Get("Authorization"))
	if len(pieces) != 2 {
		return ""
	}
	authHeaderSchemeUpper := strings.ToUpper(pieces[0])
	for _, authScheme := range allowedAuthSchemesUpper {
		if authHeaderSchemeUpper == authScheme {
			return pieces[1]
		}
	}
	return ""
}

type certInfo struct {
	Algorithm string `json:"algorithm"`
	Exponent  string `json:"exponent"`
	KeyID     string `json:"keyid"`
	Modulus   string `json:"modulus"`
}

type certsList struct {
	KeyValues []*certInfo `json:"keyvalues"`
}

// maxAge parses Cache-Control header value and extracts max-age (in seconds)
func maxAge(s string) int {
	match := maxAgePattern.FindStringSubmatch(s)
	if len(match) != 2 {
		return 0
	}
	if maxAge, err := strconv.Atoi(match[1]); err == nil {
		return maxAge
	}
	return 0
}

// certExpirationTime computes a cert freshness based on Cache-Control
// and Age or Expires headers of h.
//
// Returns 0 if one of the required headers is not present or cert lifetime
// is expired.
func certExpirationTime(h http.Header) time.Duration {
	// http://www.w3.org/Protocols/rfc2616/rfc2616-sec4.html#sec4.2 indicates only
	// a comma-separated header is valid, so it should be fine to split this on
	// commas.

	var max int
	for _, entry := range strings.Split(h.Get("Cache-Control"), ",") {
		max = maxAge(entry)
		if max > 0 {
			break
		}
	}
	if max <= 0 {
		return 0
	}

	if ageHeader := h.Get("Age"); ageHeader != "" {
		age, err := strconv.Atoi(ageHeader)
		if err != nil {
			return 0
		}

		remainingTime := max - age
		if remainingTime <= 0 {
			return 0
		}
		return time.Duration(remainingTime) * time.Second
	}
	if expiresHeader := h.Get("Expires"); expiresHeader != "" {
		if dateHeader := h.Get("Date"); dateHeader != "" {
			date, err := time.Parse(time.RFC1123, dateHeader)
			if err != nil {
				return 0
			}
			expires, err := time.Parse(time.RFC1123, expiresHeader)
			if err != nil {
				return 0
			}
			return expires.Sub(date)
		}
	}
	return 0
}

// cachedCerts fetches public certificates info from DefaultCertURI and
// caches it for the duration specified in Age header of a response.
func cachedCerts(c context.Context) (*certsList, error) {
	namespacedContext, err := appengine.Namespace(c, certNamespace)
	if err != nil {
		return nil, err
	}

	var certs *certsList

	_, err = memcache.JSON.Get(namespacedContext, DefaultCertURI, &certs)
	if err == nil {
		return certs, nil
	}

	// Cache miss or server error.
	// If any error other than cache miss, it's proably not a good time
	// to use memcache.
	var cacheResults = err == memcache.ErrCacheMiss
	if !cacheResults {
		log.Debugf(c, "%s", err.Error())
	}

	log.Debugf(c, "Fetching provider certs from: %s", DefaultCertURI)
	resp, err := newHTTPClient(c).Get(DefaultCertURI)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Could not reach Cert URI or bad response.")
	}

	certBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(certBytes, &certs)
	if err != nil {
		return nil, err
	}

	if cacheResults {
		expiration := certExpirationTime(resp.Header)
		if expiration > 0 {
			item := &memcache.Item{
				Key:        DefaultCertURI,
				Value:      certBytes,
				Expiration: expiration,
			}
			err = memcache.Set(namespacedContext, item)
			if err != nil {
				log.Errorf(c, "Error adding Certs to memcache: %v", err)
			}
		}
	}
	return certs, nil
}

type signedJWTHeader struct {
	Algorithm string `json:"alg"`
}

type signedJWT struct {
	Audience string `json:"aud"`
	ClientID string `json:"azp"`
	Subject  string `json:"sub"`
	Email    string `json:"email"`
	Expires  int64  `json:"exp"`
	IssuedAt int64  `json:"iat"`
	Issuer   string `json:"iss"`
}

// addBase64Pad pads s to be a valid base64-encoded string.
func addBase64Pad(s string) string {
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return s
}

// base64ToBig converts base64-encoded string to a big int.
// Returns error if the encoding is invalid.
func base64ToBig(s string) (*big.Int, error) {
	b, err := base64.StdEncoding.DecodeString(addBase64Pad(s))
	if err != nil {
		return nil, err
	}
	z := big.NewInt(0)
	z.SetBytes(b)
	return z, nil
}

// zeroPad prepends 0s to b so that length of the returned slice is size.
func zeroPad(b []byte, size int) []byte {
	padded := make([]byte, size-len(b), size)
	return append(padded, b...)
}

// contains returns true if value is one of the items of strList.
func contains(strList []string, value string) bool {
	for _, choice := range strList {
		if choice == value {
			return true
		}
	}
	return false
}

// verifySignedJWT decodes and verifies JWT token string.
//
// Verification is based on
//   - a certificate exponent and modulus
//   - expiration and issue timestamps ("exp" and "iat" fields)
//
// This method expects JWT token string to be in the standard format, e.g. as
// read from Authorization request header: "<header>.<payload>.<signature>",
// where all segments are encoded with URL-base64.
//
// The caller is responsible for performing further token verification.
// (Issuer, Audience, ClientID, etc.)
//
// NOTE: do not call this function directly, use jwtParser() instead.
func verifySignedJWT(c context.Context, jwt string, now int64) (*signedJWT, error) {
	segments := strings.Split(jwt, ".")
	if len(segments) != 3 {
		return nil, fmt.Errorf("Wrong number of segments in token: %s", jwt)
	}

	// Check that header (first segment) is valid
	headerBytes, err := base64.URLEncoding.DecodeString(addBase64Pad(segments[0]))
	if err != nil {
		return nil, err
	}
	var header signedJWTHeader
	err = json.Unmarshal(headerBytes, &header)
	if err != nil {
		return nil, err
	}
	if header.Algorithm != "RS256" {
		return nil, fmt.Errorf("Unexpected encryption algorithm: %s", header.Algorithm)
	}

	// Check that token (second segment) is valid
	tokenBytes, err := base64.URLEncoding.DecodeString(addBase64Pad(segments[1]))
	if err != nil {
		return nil, err
	}
	var token signedJWT
	err = json.Unmarshal(tokenBytes, &token)
	if err != nil {
		return nil, err
	}

	// Get current certs
	certs, err := cachedCerts(c)
	if err != nil {
		return nil, err
	}

	signatureBytes, err := base64.URLEncoding.DecodeString(addBase64Pad(segments[2]))
	if err != nil {
		return nil, err
	}
	signature := big.NewInt(0)
	signature.SetBytes(signatureBytes)

	signed := []byte(fmt.Sprintf("%s.%s", segments[0], segments[1]))
	h := sha256.New()
	h.Write(signed)
	signatureHash := h.Sum(nil)
	if len(signatureHash) < 32 {
		signatureHash = zeroPad(signatureHash, 32)
	}

	z := big.NewInt(0)
	verified := false
	for _, cert := range certs.KeyValues {
		exponent, err := base64ToBig(cert.Exponent)
		if err != nil {
			return nil, err
		}
		modulus, err := base64ToBig(cert.Modulus)
		if err != nil {
			return nil, err
		}
		signatureHashFromCert := z.Exp(signature, exponent, modulus).Bytes()
		// Only consider last 32 bytes
		if len(signatureHashFromCert) > 32 {
			firstIndex := len(signatureHashFromCert) - 32
			signatureHashFromCert = signatureHashFromCert[firstIndex:]
		} else if len(signatureHashFromCert) < 32 {
			signatureHashFromCert = zeroPad(signatureHashFromCert, 32)
		}
		verified = bytes.Equal(signatureHash, signatureHashFromCert)
		if verified {
			break
		}
	}

	if !verified {
		return nil, fmt.Errorf("Invalid token signature: %s", jwt)
	}

	// Check time
	if token.IssuedAt == 0 {
		return nil, fmt.Errorf("Invalid iat value in token: %s", tokenBytes)
	}
	earliest := token.IssuedAt - clockSkewSecs
	if now < earliest {
		return nil, fmt.Errorf("Token used too early, %d < %d: %s", now, earliest, tokenBytes)
	}

	if token.Expires == 0 {
		return nil, fmt.Errorf("Invalid exp value in token: %s", tokenBytes)
	} else if token.Expires >= now+maxTokenLifetimeSecs {
		return nil, fmt.Errorf("exp value is too far in the future: %s", tokenBytes)
	}
	latest := token.Expires + clockSkewSecs
	if now > latest {
		return nil, fmt.Errorf("Token used too late, %d > %d: %s", now, latest, tokenBytes)
	}

	return &token, nil
}

// verifyParsedToken performs further verification of a parsed JWT token and
// checks for the validity of Issuer, Audience, ClientID and Email fields.
//
// Returns true if token passes verification and can be accepted as indicated
// by audiences and clientIDs args.
func verifyParsedToken(c context.Context, token signedJWT, audiences []string, clientIDs []string) bool {
	// Verify the issuer.
	if token.Issuer != "accounts.google.com" {
		log.Warningf(c, "Issuer was not valid: %s", token.Issuer)
		return false
	}

	// Check audiences.
	if token.Audience == "" {
		log.Warningf(c, "Invalid aud value in token")
		return false
	}

	if token.ClientID == "" {
		log.Warningf(c, "Invalid azp value in token")
		return false
	}

	// This is only needed if Audience and ClientID differ, which (currently) only
	// happens on Android. In the case they are equal, we only need the ClientID to
	// be in the listed of accepted Client IDs.
	if token.ClientID != token.Audience && !contains(audiences, token.Audience) {
		log.Warningf(c, "Audience not allowed: %s", token.Audience)
		return false
	}

	// Check allowed client IDs.
	if len(clientIDs) == 0 {
		log.Warningf(c, "No allowed client IDs specified. ID token cannot be verified.")
		return false
	} else if !contains(clientIDs, token.ClientID) {
		log.Warningf(c, "Client ID is not allowed: %s", token.ClientID)
		return false
	}

	if token.Email == "" {
		log.Warningf(c, "Invalid email value in token")
		return false
	}

	return true
}

// currentIDTokenUser returns "appengine/user".User object if provided JWT token
// was successfully decoded and passed all verifications.
func currentIDTokenUser(c context.Context, jwt string, audiences []string, clientIDs []string, now int64) (*user.User, error) {
	parsedToken, err := jwtParser(c, jwt, now)
	if err != nil {
		return nil, err
	}

	if verifyParsedToken(c, *parsedToken, audiences, clientIDs) {
		return &user.User{
			ID:       parsedToken.Subject,
			Email:    parsedToken.Email,
			ClientID: parsedToken.ClientID,
		}, nil
	}

	return nil, errors.New("No ID token user found.")
}

// CurrentBearerTokenScope compares given scopes and clientIDs with those in c.
//
// Both scopes and clientIDs args must have at least one element.
//
// Returns a single scope (one of provided scopes) if the two conditions are met:
//   - it is found in Context c
//   - client ID on that scope matches one of clientIDs in the args
func CurrentBearerTokenScope(c context.Context, scopes []string, clientIDs []string) (string, error) {
	auth := authenticator(c)
	if auth == nil {
		return "", errNoAuthenticator
	}
	for _, scope := range scopes {
		currentClientID, err := auth.CurrentOAuthClientID(c, scope)
		if err != nil {
			continue
		}

		for _, id := range clientIDs {
			if id == currentClientID {
				return scope, nil
			}
		}

		// If none of the client IDs matches, return nil
		log.Debugf(c, "Couldn't find current client ID %q in %v", currentClientID, clientIDs)
		return "", errors.New("Mismatched Client ID")
	}
	// No client ID found for any of the scopes
	return "", errors.New("No valid scope")
}

// CurrentBearerTokenUser returns a user associated with the request which is
// expected to have a Bearer token.
//
// Both scopes and clientIDs must have at least one element.
//
// Returns an error if the client did not make a valid request, or none of
// clientIDs are allowed to make requests, or user did not authorize any of
// the scopes.
func CurrentBearerTokenUser(c context.Context, scopes []string, clientIDs []string) (*user.User, error) {
	auth := authenticator(c)
	if auth == nil {
		return nil, errNoAuthenticator
	}
	scope, err := CurrentBearerTokenScope(c, scopes, clientIDs)
	if err != nil {
		return nil, err
	}

	return auth.CurrentOAuthUser(c, scope)
}

// CurrentUser checks for both JWT and Bearer tokens.
//
// It first tries to decode and verify JWT token (if conditions are met)
// and falls back to Bearer token.
//
// The returned user will have only ID, Email and ClientID fields set.
// User.ID is a Google Account ID, which is different from GAE user ID.
// For more info on User.ID see 'sub' claim description on
// https://developers.google.com/identity/protocols/OpenIDConnect#obtainuserinfo
func CurrentUser(c context.Context, scopes []string, audiences []string, clientIDs []string) (*user.User, error) {
	// The user hasn't provided any information to allow us to parse either
	// an ID token or a Bearer token.
	if len(scopes) == 0 && len(audiences) == 0 && len(clientIDs) == 0 {
		return nil, errors.New("no client ID or scope info provided.")
	}
	r := HTTPRequest(c)
	if r == nil {
		return nil, errNoRequest
	}

	token := parseToken(r)
	if token == "" {
		return nil, errors.New("No token in the current context.")
	}

	// If the only scope is the email scope, check an ID token. Alternatively,
	// we dould check if token starts with "ya29." or "1/" to decide that it
	// is a Bearer token. This is what is done in Java.
	if len(scopes) == 1 && scopes[0] == EmailScope && len(clientIDs) > 0 {
		log.Debugf(c, "Checking for ID token.")
		now := currentUTC().Unix()
		u, err := currentIDTokenUser(c, token, audiences, clientIDs, now)
		// Only return in case of success, else pass along and try
		// parsing Bearer token.
		if err == nil {
			return u, err
		}
	}

	log.Debugf(c, "Checking for Bearer token.")
	return CurrentBearerTokenUser(c, scopes, clientIDs)
}

func init() {
	if appengine.IsDevAppServer() {
		AuthenticatorFactory = tokeninfoAuthenticatorFactory
	} else {
		AuthenticatorFactory = cachingAuthenticatorFactory
	}
}
