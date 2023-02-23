package require

import (
	"reflect"
	"testing"
)

func NoError(t testing.TB, err error, args ...any) {
	if err != nil {
		t.Helper()
		t.Fatal(append([]any{err}, args...)...)
	}
}

func Equal[E any](t testing.TB, expected, got E, args ...any) {
	if !reflect.DeepEqual(expected, got) {
		t.Helper()
		t.Fatal(append([]any{expected, "is not", got}, args...)...)
	}
}
