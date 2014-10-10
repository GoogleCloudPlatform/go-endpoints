// Testing utils

package endpoints

import (
	"errors"
	"io"
	"net/http"
	"reflect"
	"testing"

	"appengine/aetest"
)

// verifyPairs loops over ab slice and calls reflect.DeepEqual() on each pair.
// Expects even number of ab args.
// When the order matters, "a" (first element of ab pair) is normally "actual"
// and "b" (second element) is expected.
func verifyPairs(t *testing.T, ab ...interface{}) {
	lenAb := len(ab)
	if lenAb%2 != 0 {
		t.Fatalf("verifyPairs: odd number of ab args (%d)", lenAb)
		return
	}
	for i := 0; i < lenAb; i += 2 {
		if !reflect.DeepEqual(ab[i], ab[i+1]) {
			t.Errorf("verifyPairs(%d): ab[%d] != ab[%d] (%#v != %#v)",
				i/2, i, i+1, ab[i], ab[i+1])
		}
	}
}

// newTestRequest creates a new request using aetest package.
// last return value is a closer function.
func newTestRequest(t *testing.T, method, path string, body io.Reader) (
	*http.Request,
	aetest.Instance,
	func(),
) {
	inst, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatalf("Failed to create instance: %v", err)
	}

	req, err := inst.NewRequest(method, path, body)
	if err != nil {
		t.Fatalf("Failed to create req: %v", err)
	}

	return req, inst, func() { inst.Close() }
}

func newTestRoundTripper(resp ...*http.Response) *TestRoundTripper {
	rt := &TestRoundTripper{}
	rt.Add(resp...)
	return rt
}

type TestRoundTripper struct {
	reqs      []*http.Request
	responses []*http.Response
	next      int
}

func (rt *TestRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	rt.reqs = append(rt.reqs, r)
	if rt.next+1 > len(rt.responses) {
		return nil, errors.New("TestRoundTripper ran out of responses")
	}
	resp := rt.responses[rt.next]
	rt.next++
	return resp, nil
}

// Add appends another response(s)
func (rt *TestRoundTripper) Add(resp ...*http.Response) {
	rt.responses = append(rt.responses, resp...)
}

// Count returns the number of responses which have been served so far.
func (rt *TestRoundTripper) Count() int {
	return rt.next
}

func (rt *TestRoundTripper) Requests() []*http.Request {
	return rt.reqs
}

func (rt *TestRoundTripper) Responses() []*http.Response {
	return rt.responses
}
