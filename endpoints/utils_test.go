// Testing utils

package endpoints

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

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
		assertEquals(t, ab[i], ab[i+1])
	}
}

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

// assertEquals compares a and b using reflect.DeepEqual.
// When the order matters, "a" is normally an actual value, "b" is expected.
func assertEquals(t *testing.T, a, b interface{}) {
	if !reflect.DeepEqual(a, b) {
		fail(t, "Expected %#+v (%T) to equal %#+v (%T)", a, a, b, b)
	}
}
