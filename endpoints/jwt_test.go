package endpoints

import (
	"errors"
	"testing"
	"time"

	"appengine/memcache"

	mc_pb "appengine_internal/memcache"
	"code.google.com/p/goprotobuf/proto"

	tu "github.com/crhym3/aegot/testutils"
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

func stubMemcacheGetCerts() func() {
	stub := func(in, out proto.Message, _ *tu.RpcCallOptions) error {
		req := in.(*mc_pb.MemcacheGetRequest)
		if req.GetNameSpace() == certNamespace &&
			len(req.Key) == 1 && string(req.Key[0]) == DefaultCertUri {

			item := &mc_pb.MemcacheGetResponse_Item{
				Key:   req.Key[0],
				Value: []byte(googCerts),
			}
			resp := out.(*mc_pb.MemcacheGetResponse)
			resp.Item = []*mc_pb.MemcacheGetResponse_Item{item}
			return nil
		}
		return memcache.ErrCacheMiss
	}
	return tu.RegisterAPIOverride("memcache", "Get", stub)
}

func TestVerifySignedJwt(t *testing.T) {
	defer stubMemcacheGetCerts()()
	r, deleteAppengineContext := tu.NewTestRequest("GET", "/", nil)
	defer deleteAppengineContext()

	tts := []struct {
		token    string
		now      time.Time
		expected *signedJWT
	}{
		{jwtValidTokenString, jwtValidTokenTime, &jwtValidTokenObject},
		{jwtValidTokenString, jwtValidTokenTime.Add(time.Hour * 24), nil},
		{jwtValidTokenString, jwtValidTokenTime.Add(-time.Hour * 24), nil},
		{jwtInvalidKeyToken, jwtValidTokenTime, nil},
		{jwtInvalidAlgToken, jwtValidTokenTime, nil},
		{"invalid.token", jwtValidTokenTime, nil},
		{"another.invalid.token", jwtValidTokenTime, nil},
	}

	c := NewContext(r)

	for i, tt := range tts {
		jwt, err := verifySignedJwt(c, tt.token, tt.now.Unix())
		switch {
		case err != nil && tt.expected != nil:
			t.Errorf("%d: didn't expect error: %v", i, err)
		case err == nil && tt.expected == nil:
			t.Errorf("%d: expected error, got: %#v", i, jwt)
		case err == nil && tt.expected != nil:
			assertEquals(t, i, jwt, tt.expected)
		}
	}
}

func TestVerifyParsedToken(t *testing.T) {
	const (
		goog     = "accounts.google.com"
		clientId = "my-client-id"
		email    = "dude@gmail.com"
	)
	audiences := []string{clientId, "hello-android"}
	clientIds := []string{clientId}

	tts := []struct {
		issuer, audience, clientId, email string
		valid                             bool
	}{
		{goog, clientId, clientId, email, true},
		{goog, "hello-android", clientId, email, true},
		{goog, "invalid", clientId, email, false},
		{goog, clientId, "invalid", email, false},
		{goog, clientId, clientId, "", false},
		{"", clientId, clientId, email, false},
	}

	r, deleteCtx := tu.NewTestRequest("GET", "/", nil)
	defer deleteCtx()

	c := NewContext(r)

	for i, tt := range tts {
		jwt := signedJWT{
			Issuer:   tt.issuer,
			Audience: tt.audience,
			ClientID: tt.clientId,
			Email:    tt.email,
		}
		out := verifyParsedToken(c, jwt, audiences, clientIds)
		if tt.valid != out {
			t.Errorf("%d: expected token to be valid? %v, got: %v",
				i, tt.valid, out)
		}
	}
}

func TestCurrentIDTokenUser(t *testing.T) {
	jwtOrigParser := jwtParser
	defer func() {
		jwtParser = jwtOrigParser
	}()
	r, deleteAppengineContext := tu.NewTestRequest("GET", "/", nil)
	defer deleteAppengineContext()
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
		token         *signedJWT
		expectedEmail string
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
		case tt.expectedEmail != "" && err != nil:
			t.Errorf("%d: unexpected error: %v", i, err)
		case tt.expectedEmail == "" && err == nil:
			t.Errorf("%d: expected error, got: %#v", i, user)
		case err == nil && tt.expectedEmail != user.Email:
			t.Errorf("%d: expected %q, got %q", i, tt.expectedEmail, user.Email)
		}
	}
}
