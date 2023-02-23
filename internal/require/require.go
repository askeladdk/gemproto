package require

import (
	"errors"
	"reflect"
	"testing"
)

func NoError(t testing.TB, err error, args ...any) {
	if err != nil {
		t.Helper()
		t.Fatal(append([]any{err}, args...)...)
	}
}

func ErrorIs(t testing.TB, err, target error, args ...any) {
	if !errors.Is(err, target) {
		t.Helper()
		t.Fatal(append([]any{err, "is not", target}, args...)...)
	}
}

func Equal[E any](t testing.TB, expected, got E, args ...any) {
	if !reflect.DeepEqual(expected, got) {
		t.Helper()
		t.Fatal(append([]any{expected, "is not", got}, args...)...)
	}
}

func True(t testing.TB, cond bool, args ...any) {
	if !cond {
		t.Helper()
		t.Fatal(append([]any{"is false"}, args...)...)
	}
}
