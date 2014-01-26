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
	"sync"
	"time"

	"appengine"
	"appengine/memcache"
	"appengine/urlfetch"
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
	ctxsMu                  sync.Mutex
	ctxs                    = make(map[*http.Request]Context)
	allowedAuthSchemesUpper = [2]string{"OAUTH", "BEARER"}
	certNamespace           = "__verify_jwt"
	clockSkewSecs           = int64(300)   // 5 minutes in seconds
	maxTokenLifetimeSecs    = int64(86400) // 1 day in seconds
	maxAgePattern           = regexp.MustCompile(`\s*max-age\s*=\s*(\d+)\s*`)

	// This is a variable on purpose: can be stubbed with a different (fake)
	// implementation during tests.
	//
	// endpoints package code should always call jwtParser()
	// instead of directly invoking verifySignedJwt().
	jwtParser = verifySignedJwt

	// currentUTC returns current time in UTC.
	// This is a variable on purpose to be able to stub during testing.
	currentUTC = func() time.Time {
		return time.Now().UTC()
	}

	// ContextFactory takes an in-flight HTTP request and creates a new
	// context.
	//
	// It is a variable on purpose. You can set it to a stub implementation
	// in tests.
	ContextFactory func(*http.Request) Context
)

// Context represents the context of an in-flight API request.
// It embeds appengine.Context so you can use it with any other appengine/*
// package methods.
type Context interface {
	appengine.Context

	// HttpRequest returns the request associated with this context.
	HttpRequest() *http.Request

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

// destroyContext removes all references to a Context c so that GC can
// do its thing and collect the garbage.
func destroyContext(c Context) {
	ctxsMu.Lock()
	defer ctxsMu.Unlock()
	delete(ctxs, c.HttpRequest())
}

// getToken looks for Authorization header and returns a token.
//
// Returns empty string if req does not contain authorization header
// or its value is not prefixed with allowedAuthSchemesUpper.
func getToken(req *http.Request) string {
	// TODO(dhermes): Allow a struct with access_token and bearer_token
	//                fields here as well.
	pieces := strings.Fields(req.Header.Get("Authorization"))
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

// getMaxAge parses Cache-Control header value and extracts max-age (in seconds)
func getMaxAge(s string) int {
	match := maxAgePattern.FindStringSubmatch(s)
	if len(match) != 2 {
		return 0
	}
	if maxAge, err := strconv.Atoi(match[1]); err == nil {
		return maxAge
	}
	return 0
}

// getCertExpirationTime computes a cert freshness based on Cache-Control
// and Age headers of h.
//
// Returns 0 if one of the required headers is not present or cert lifetime
// is expired.
func getCertExpirationTime(h http.Header) time.Duration {
	// http://www.w3.org/Protocols/rfc2616/rfc2616-sec4.html#sec4.2 indicates only
	// a comma-separated header is valid, so it should be fine to split this on
	// commas.
	var maxAge int
	for _, entry := range strings.Split(h.Get("Cache-Control"), ",") {
		maxAge = getMaxAge(entry)
		if maxAge > 0 {
			break
		}
	}
	if maxAge <= 0 {
		return 0
	}

	age, err := strconv.Atoi(h.Get("Age"))
	if err != nil {
		return 0
	}

	remainingTime := maxAge - age
	if remainingTime <= 0 {
		return 0
	}

	return time.Duration(remainingTime) * time.Second
}

// getCachedCerts fetches public certificates info from DefaultCertUri and
// caches it for the duration specified in Age header of a response.
func getCachedCerts(c Context) (*certsList, error) {
	namespacedContext, err := appengine.Namespace(c, certNamespace)
	if err != nil {
		return nil, err
	}

	var certs *certsList

	_, err = memcache.JSON.Get(namespacedContext, DefaultCertUri, &certs)
	if err == nil {
		return certs, nil
	}

	// Cache miss or server error.
	// If any error other than cache miss, it's proably not a good time
	// to use memcache.
	var cacheResults = err == memcache.ErrCacheMiss
	if !cacheResults {
		c.Debugf(err.Error())
	}

	client := urlfetch.Client(c)
	resp, err := client.Get(DefaultCertUri)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, errors.New("Could not reach Cert URI")
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
		expiration := getCertExpirationTime(resp.Header)
		if expiration > 0 {
			item := &memcache.Item{
				Key:        DefaultCertUri,
				Value:      certBytes,
				Expiration: expiration,
			}
			err = memcache.Set(namespacedContext, item)
			if err != nil {
				c.Errorf("Error adding Certs to memcache: %v", err)
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

// verifySignedJwt decodes and verifies JWT token string.
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
func verifySignedJwt(c Context, jwt string, now int64) (*signedJWT, error) {
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
	certs, err := getCachedCerts(c)
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
func verifyParsedToken(c Context, token signedJWT, audiences []string, clientIDs []string) bool {
	// Verify the issuer.
	if token.Issuer != "accounts.google.com" {
		c.Warningf("Issuer was not valid: %s", token.Issuer)
		return false
	}

	// Check audiences.
	if token.Audience == "" {
		c.Warningf("Invalid aud value in token")
		return false
	}

	if token.ClientID == "" {
		c.Warningf("Invalid azp value in token")
		return false
	}

	// This is only needed if Audience and ClientID differ, which (currently) only
	// happens on Android. In the case they are equal, we only need the ClientID to
	// be in the listed of accepted Client IDs.
	if token.ClientID != token.Audience && !contains(audiences, token.Audience) {
		c.Warningf("Audience not allowed: %s", token.Audience)
		return false
	}

	// Check allowed client IDs.
	if len(clientIDs) == 0 {
		c.Warningf("No allowed client IDs specified. ID token cannot be verified.")
		return false
	} else if !contains(clientIDs, token.ClientID) {
		c.Warningf("Client ID is not allowed: %s", token.ClientID)
		return false
	}

	if token.Email == "" {
		c.Warningf("Invalid email value in token")
		return false
	}

	return true
}

// currentIDTokenUser returns "appengine/user".User object if provided JWT token
// was successfully decoded and passed all verifications.
//
// Currently, only Email field will be set in case of success.
func currentIDTokenUser(c Context, jwt string, audiences []string, clientIDs []string, now int64) (*user.User, error) {
	parsedToken, err := jwtParser(c, jwt, now)
	if err != nil {
		return nil, err
	}

	if verifyParsedToken(c, *parsedToken, audiences, clientIDs) {
		return &user.User{
			Email: parsedToken.Email,
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

// CurrentBearerTokenUser returns a user associated with the request which is
// expected to have a Bearer token.
//
// Both scopes and clientIDs must have at least one element.
//
// Returns an error if the client did not make a valid request, or none of
// clientIDs are allowed to make requests, or user did not authorize any of
// the scopes.
func CurrentBearerTokenUser(c Context, scopes []string, clientIDs []string) (*user.User, error) {
	scope, err := CurrentBearerTokenScope(c, scopes, clientIDs)
	if err != nil {
		return nil, err
	}

	return c.CurrentOAuthUser(scope)
}

// CurrentUser checks for both JWT and Bearer tokens.
//
// It first tries to decode and verify JWT token (if conditions are met)
// and falls back to Bearer token.
//
// NOTE: Currently, returned user will have only Email field set when JWT is used.
func CurrentUser(c Context, scopes []string, audiences []string, clientIDs []string) (*user.User, error) {
	// The user hasn't provided any information to allow us to parse either
	// an ID token or a Bearer token.
	if len(scopes) == 0 && len(audiences) == 0 && len(clientIDs) == 0 {
		return nil, errors.New("No client ID or scope info provided.")
	}

	token := getToken(c.HttpRequest())
	if token == "" {
		return nil, errors.New("No token in the current context.")
	}

	// If the only scope is the email scope, check an ID token. Alternatively,
	// we dould check if token starts with "ya29." or "1/" to decide that it
	// is a Bearer token. This is what is done in Java.
	if len(scopes) == 1 && scopes[0] == EmailScope && len(clientIDs) > 0 {
		c.Infof("Checking for ID token.")
		now := currentUTC().Unix()
		u, err := currentIDTokenUser(c, token, audiences, clientIDs, now)
		// Only return in case of success, else pass along and try
		// parsing Bearer token.
		if err == nil {
			return u, err
		}
	}

	c.Infof("Checking for Bearer token.")
	return CurrentBearerTokenUser(c, scopes, clientIDs)
}

func init() {
	if appengine.IsDevAppServer() {
		ContextFactory = tokeninfoContextFactory
	} else {
		ContextFactory = cachingContextFactory
	}
}
