package endpoints

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"appengine"
	"appengine/memcache"
)

var jwtValidTokenObject = signedJWT{
	Audience: "my-client-id",
	ClientID: "hello-android",
	Email:    "dude@gmail.com",
	Expires:  1370352252,
	IssuedAt: 1370348652,
	Issuer:   "accounts.google.com",
}

// jwtValidTokenTime is a "timestamp" at which jwtValidTokenObject is valid
// (e.g. not expired or something)
var jwtValidTokenTime = time.Date(2013, 6, 4, 13, 24, 15, 0, time.UTC)

// header: {"alg": "RS256", "typ": "JWT"}
// payload:
// 	{
// 		"aud": "my-client-id",
// 		"azp": "hello-android",
// 		"email": "dude@gmail.com",
// 		"exp": 1370352252,
// 		"iat": 1370348652,
// 		"iss": "accounts.google.com"
// 	}
// issued at 2013-06-04 14:24:12 UTC
// expires at 2013-06-04 15:24:12 UTC
const jwtValidTokenString = ("eyJhbGciOiAiUlMyNTYiLCAidHlwIjogIkpXVCJ9." +
	"eyJhdWQiOiAibXktY2xpZW50LWlkIiwgImlzcyI6ICJhY2NvdW50cy5nb29nbGUuY29tIiwg" +
	"ImV4cCI6IDEzNzAzNTIyNTIsICJhenAiOiAiaGVsbG8tYW5kcm9pZCIsICJpYXQiOiAxMzcw" +
	"MzQ4NjUyLCAiZW1haWwiOiAiZHVkZUBnbWFpbC5jb20ifQ." +
	"sv7l0v_u6DmVe7s-hg8Q5LOYXNCdUBR7efnvQ4ns6IfBFZ71yPvWfwOYqZuYGQ0a9V5CfR0r" +
	"TfNlXVEpW5NE9rZy8hFiZkHBE30yPDti6PUUtT1bZST1VPFnIvSHobcUj-QPBTRC1Df86Vv0" +
	"Jmx4yowL1z3Yhe0Zh1WcvUUPG9sKJt8_-qKAv9QeeCMveBYpRSh6JvoU_qUKxPTjOOLvQiqV" +
	"4NiNjJ3sDN0P4BHJc3VcqB-SFd7kMRgQy1Fq-NHKN5-T2x4gxPwUy9GvHOftxY47B1NtJ9Q5" +
	"KtSui9uXdyBNJnt0xcIT5CcQYkVLoeCldxpSfwfA2kyfuJKBeiQnSA")

// same header and payload, only encoded with a random private key
// (generate one with "openssl genrsa 2048 > dummy.key")
const jwtInvalidKeyToken = ("eyJhbGciOiAiUlMyNTYiLCAidHlwIjogIkpXVCJ9." +
	"eyJhdWQiOiAibXktY2xpZW50LWlkIiwgImlzcyI6ICJhY2NvdW50cy5nb29nbGUuY29tIiwg" +
	"ImV4cCI6IDEzNzAzNTIyNTIsICJhenAiOiAiaGVsbG8tYW5kcm9pZCIsICJpYXQiOiAxMzcw" +
	"MzQ4NjUyLCAiZW1haWwiOiAiZHVkZUBnbWFpbC5jb20ifQ." +
	"PatagaopzqOe_LqM4rddJHKaZ-l2bacN3Lsj2t15c_iZRzjgFXlC6CsR64SaHSdC-wxde3wu" +
	"OKKRWZPZA2Zr03TRUMB_iLJDs2Gg4dUsEsVrbZkTzrGcmejrHIIA1wP0hIM1COBIo6bYr9Vz" +
	"UBDBR4tlq8kRgNdCmHXRrR1u4ITSFin3skRE6xkIFXmswI4lzpfWGD2f4jsnH8HDu5K-9X3Q" +
	"OhUgKUL9Hlz5z2PtLMGl0xXzFNSPoWAPPNZkNJlPUfLKL7QnJdWu_ieQ1L0xUWHcGvb76lgD" +
	"AbihLmbYrX0DeIMl6a_n4wKLRzoVv7qz9KlH-RbMjebudwPRnU-yeg")

// header: {"alg": "HS256", "typ": "JWT"}
// same payload.
const jwtInvalidAlgToken = ("eyJhbGciOiAiSFMyNTYiLCAidHlwIjogIkpXVCJ9." +
	"Intcblx0XCJhdWRcIjogXCJteS1jbGllbnQtaWRcIlxuXHRcImF6cFwiOiBcImhlbGxvLWFuZ" +
	"HJvaWRcIlxuXHRcImVtYWlsXCI6IFwiZHVkZUBnbWFpbC5jb21cIlxuXHRcImV4cFwiOiAxMz" +
	"cwMzUyMjUyXG5cdFwiaWF0XCI6IDEzNzAzNDg2NTJcblx0XCJpc3NcIjogXCJhY2NvdW50cy5" +
	"nb29nbGUuY29tXCJcbn0i." +
	"ylh77ZOrr_Bkd3iFZRrQNcf_GCjCpUtcdhWz3AOLWUA")

// Private key for testing.
// Used to create exponent, modulus and sign JWT token.
const privateKeyPem = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA4ej0p7bQ7L/r4rVGUz9RN4VQWoej1Bg1mYWIDYslvKrk1gpj
7wZgkdmM7oVK2OfgrSj/FCTkInKPqaCR0gD7K80q+mLBrN3PUkDrJQZpvRZIff3/
xmVU1WeruQLFJjnFb2dqu0s/FY/2kWiJtBCakXvXEOb7zfbINuayL+MSsCGSdVYs
SliS5qQpgyDap+8b5fpXZVJkq92hrcNtbkg7hCYUJczt8n9hcCTJCfUpApvaFQ18
pe+zpyl4+WzkP66I28hniMQyUlA1hBiskT7qiouq0m8IOodhv2fagSZKjOTTU2xk
SBc//fy3ZpsL7WqgsZS7Q+0VRK8gKfqkxg5OYQIDAQABAoIBAQDGGHzQxGKX+ANk
nQi53v/c6632dJKYXVJC+PDAz4+bzU800Y+n/bOYsWf/kCp94XcG4Lgsdd0Gx+Zq
HD9CI1IcqqBRR2AFscsmmX6YzPLTuEKBGMW8twaYy3utlFxElMwoUEsrSWRcCA1y
nHSDzTt871c7nxCXHxuZ6Nm/XCL7Bg8uidRTSC1sQrQyKgTPhtQdYrPQ4WZ1A4J9
IisyDYmZodSNZe5P+LTJ6M1SCgH8KH9ZGIxv3diMwzNNpk3kxJc9yCnja4mjiGE2
YCNusSycU5IhZwVeCTlhQGcNeV/skfg64xkiJE34c2y2ttFbdwBTPixStGaF09nU
Z422D40BAoGBAPvVyRRsC3BF+qZdaSMFwI1yiXY7vQw5+JZh01tD28NuYdRFzjcJ
vzT2n8LFpj5ZfZFvSMLMVEFVMgQvWnN0O6xdXvGov6qlRUSGaH9u+TCPNnIldjMP
B8+xTwFMqI7uQr54wBB+Poq7dVRP+0oHb0NYAwUBXoEuvYo3c/nDoRcZAoGBAOWl
aLHjMv4CJbArzT8sPfic/8waSiLV9Ixs3Re5YREUTtnLq7LoymqB57UXJB3BNz/2
eCueuW71avlWlRtE/wXASj5jx6y5mIrlV4nZbVuyYff0QlcG+fgb6pcJQuO9DxMI
aqFGrWP3zye+LK87a6iR76dS9vRU+bHZpSVvGMKJAoGAFGt3TIKeQtJJyqeUWNSk
klORNdcOMymYMIlqG+JatXQD1rR6ThgqOt8sgRyJqFCVT++YFMOAqXOBBLnaObZZ
CFbh1fJ66BlSjoXff0W+SuOx5HuJJAa5+WtFHrPajwxeuRcNa8jwxUsB7n41wADu
UqWWSRedVBg4Ijbw3nWwYDECgYB0pLew4z4bVuvdt+HgnJA9n0EuYowVdadpTEJg
soBjNHV4msLzdNqbjrAqgz6M/n8Ztg8D2PNHMNDNJPVHjJwcR7duSTA6w2p/4k28
bvvk/45Ta3XmzlxZcZSOct3O31Cw0i2XDVc018IY5be8qendDYM08icNo7vQYkRH
504kQQKBgQDjx60zpz8ozvm1XAj0wVhi7GwXe+5lTxiLi9Fxq721WDxPMiHDW2XL
YXfFVy/9/GIMvEiGYdmarK1NW+VhWl1DC5xhDg0kvMfxplt4tynoq1uTsQTY31Mx
BeF5CT/JuNYk3bEBF0H/Q3VGO1/ggVS+YezdFbLWIRoMnLj6XCFEGg==
-----END RSA PRIVATE KEY-----`

// openssl rsa -in private.pem -noout -text
const googCerts = `{
	"keyvalues": [{
		"algorithm": "RSA",
		"modulus": "` +
	"AOHo9Ke20Oy/6+K1RlM/UTeFUFqHo9QYNZmFiA2LJbyq5NYKY+8GYJHZjO6FStjn4" +
	"K0o/xQk5CJyj6mgkdIA+yvNKvpiwazdz1JA6yUGab0WSH39/8ZlVNVnq7kCxSY5xW" +
	"9nartLPxWP9pFoibQQmpF71xDm+832yDbmsi/jErAhknVWLEpYkuakKYMg2qfvG+X" +
	"6V2VSZKvdoa3DbW5IO4QmFCXM7fJ/YXAkyQn1KQKb2hUNfKXvs6cpePls5D+uiNvI" +
	"Z4jEMlJQNYQYrJE+6oqLqtJvCDqHYb9n2oEmSozk01NsZEgXP/38t2abC+1qoLGUu" +
	"0PtFUSvICn6pMYOTmE=" + `",
		"exponent": "AQAB",
		"keyid": "goog-123"
	}]
}`

func TestverifySignedJWT(t *testing.T) {
	r, _, closer := newTestRequest(t, "GET", "/", nil)
	defer closer()
	nc, err := appengine.Namespace(appengine.NewContext(r), certNamespace)
	if err != nil {
		t.Fatal(err)
	}

	item := &memcache.Item{Key: DefaultCertURI, Value: []byte(googCerts)}
	if err := memcache.Set(nc, item); err != nil {
		t.Fatal(err)
	}

	tts := []struct {
		token string
		now   time.Time
		want  *signedJWT
	}{
		{jwtValidTokenString, jwtValidTokenTime, &jwtValidTokenObject},
		{jwtValidTokenString, jwtValidTokenTime.Add(time.Hour * 24), nil},
		{jwtValidTokenString, jwtValidTokenTime.Add(-time.Hour * 24), nil},
		{jwtInvalidKeyToken, jwtValidTokenTime, nil},
		{jwtInvalidAlgToken, jwtValidTokenTime, nil},
		{"invalid.token", jwtValidTokenTime, nil},
		{"another.invalid.token", jwtValidTokenTime, nil},
	}

	ec := NewContext(r)

	for i, tt := range tts {
		jwt, err := verifySignedJWT(ec, tt.token, tt.now.Unix())
		switch {
		case err != nil && tt.want != nil:
			t.Errorf("%d: verifySignedJWT(%q, %d) = %v; want %#v",
				i, tt.token, tt.now.Unix(), err, tt.want)
		case err == nil && tt.want == nil:
			t.Errorf("%d: verifySignedJWT(%q, %d) = %#v; want error",
				i, tt.token, tt.now.Unix(), jwt)
		case err == nil && tt.want != nil:
			if !reflect.DeepEqual(jwt, tt.want) {
				t.Errorf("%d: verifySignedJWT(%q, %d) = %v; want %#v",
					i, tt.token, tt.now.Unix(), jwt, tt.want)
			}
		}
	}
}

func TestVerifyParsedToken(t *testing.T) {
	const (
		goog     = "accounts.google.com"
		clientID = "my-client-id"
		email    = "dude@gmail.com"
	)
	audiences := []string{clientID, "hello-android"}
	clientIDs := []string{clientID}

	tts := []struct {
		issuer, audience, clientID, email string
		valid                             bool
	}{
		{goog, clientID, clientID, email, true},
		{goog, "hello-android", clientID, email, true},
		{goog, "invalid", clientID, email, false},
		{goog, clientID, "invalid", email, false},
		{goog, clientID, clientID, "", false},
		{"", clientID, clientID, email, false},
	}

	r, _, closer := newTestRequest(t, "GET", "/", nil)
	defer closer()
	c := NewContext(r)

	for i, tt := range tts {
		jwt := signedJWT{
			Issuer:   tt.issuer,
			Audience: tt.audience,
			ClientID: tt.clientID,
			Email:    tt.email,
		}
		res := verifyParsedToken(c, jwt, audiences, clientIDs)
		if res != tt.valid {
			t.Errorf("%d: verifyParsedToken(%#v, %v, %v) = %v; want %v",
				i, jwt, audiences, clientIDs, res, tt.valid)
		}
	}
}

func TestCurrentIDTokenUser(t *testing.T) {
	jwtOrigParser := jwtParser
	defer func() {
		jwtParser = jwtOrigParser
	}()

	r, _, closer := newTestRequest(t, "GET", "/", nil)
	defer closer()
	c := NewContext(r)

	aud := []string{jwtValidTokenObject.Audience, jwtValidTokenObject.ClientID}
	azp := []string{jwtValidTokenObject.ClientID}

	jwtUnacceptedToken := signedJWT{
		Audience: "my-other-client-id",
		ClientID: "my-other-client-id",
		Email:    "me@gmail.com",
		Expires:  1370352252,
		IssuedAt: 1370348652,
		Issuer:   "accounts.google.com",
	}

	tts := []struct {
		token     *signedJWT
		wantEmail string
	}{
		{&jwtValidTokenObject, jwtValidTokenObject.Email},
		{&jwtUnacceptedToken, ""},
		{nil, ""},
	}

	var currToken *signedJWT

	jwtParser = func(Context, string, int64) (*signedJWT, error) {
		if currToken == nil {
			return nil, errors.New("Fake verification failed")
		}
		return currToken, nil
	}

	for i, tt := range tts {
		currToken = tt.token
		user, err := currentIDTokenUser(c,
			jwtValidTokenString, aud, azp, jwtValidTokenTime.Unix())
		switch {
		case tt.wantEmail != "" && err != nil:
			t.Errorf("%d: currentIDTokenUser(%q, %v, %v, %d) = %v; want email = %q",
				i, jwtValidTokenString, aud, azp, jwtValidTokenTime.Unix(), err, tt.wantEmail)
		case tt.wantEmail == "" && err == nil:
			t.Errorf("%d: currentIDTokenUser(%q, %v, %v, %d) = %#v; want error",
				i, jwtValidTokenString, aud, azp, jwtValidTokenTime.Unix(), user)
		case err == nil && tt.wantEmail != user.Email:
			t.Errorf("%d: currentIDTokenUser(%q, %v, %v, %d) = %#v; want email = %q",
				i, jwtValidTokenString, aud, azp, jwtValidTokenTime.Unix(), user, tt.wantEmail)
		}
	}
}
