// Testing utils

package endpoints

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"appengine/aetest"
)

// fail looks for the original caller file and line that are within endpoints
// package and appends this info to the error message.
func fail(t *testing.T, msg string, args ...interface{}) {
	const path = "endpoints"
	var (
		file string
		line int
	)
	for skip := 0; ; skip += 1 {
		if _, f, l, ok := runtime.Caller(skip); ok && strings.Contains(f, path) {
			file, line = f, l
			continue
		}
		break
	}
	_, file = filepath.Split(file)
	args = append(args, file, line)
	t.Errorf(msg+" (in %s:%d)", args...)
}

// verifyTT loops over ab slice and assertEquals on each pair.
// Expects even number of ab args.
// When the order matters, "a" (first element of ab pair) is normally "actual"
// and "b" (second element) is expected.
func verifyTT(t *testing.T, ab ...interface{}) {
	lenAb := len(ab)
	if lenAb%2 != 0 {
		fail(t, "verifyTT: odd number of ab args (%d)", lenAb)
		return
	}
	for i := 0; i < lenAb; i += 2 {
		assertEquals(t, i/2, ab[i], ab[i+1])
	}
}

// assertEquals compares a and b using reflect.DeepEqual.
// When the order matters, "a" is normally an actual value, "b" is expected.
func assertEquals(t *testing.T, pos, a, b interface{}) {
	if !reflect.DeepEqual(a, b) {
		fail(t, "%v: expected %#+v (%T) to equal %#+v (%T)", pos, a, a, b, b)
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
	rt.next += 1
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
