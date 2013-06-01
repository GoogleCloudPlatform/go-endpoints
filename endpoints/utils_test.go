// Testing utils

package endpoints

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// ttRow is a table test row. When the order matters, "a" is normally "actual"
// and "b" is expected.
type ttRow struct {
	a interface{}
	b interface{}
}

// verifyTT loops over tests and assertEquals on each ttRow.
func verifyTT(t *testing.T, tests []*ttRow) {
	for _, test := range tests {
		assertEquals(t, test.a, test.b)
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
		fail(t, "Expected %#+v to equal %#+v", a, b)
	}
}
