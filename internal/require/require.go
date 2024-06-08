package require

import (
	"errors"
	"reflect"
	"testing"
)

func NoError(tb testing.TB, err error, args ...any) {
	tb.Helper()
	if err != nil {
		tb.Fatal(append([]any{err}, args...)...)
	}
}

func ErrorIs(tb testing.TB, err, target error, args ...any) {
	tb.Helper()
	if !errors.Is(err, target) {
		tb.Fatal(append([]any{err, "is not", target}, args...)...)
	}
}

func Equal[E any](tb testing.TB, expected, got E, args ...any) {
	tb.Helper()
	if !reflect.DeepEqual(expected, got) {
		tb.Fatal(append([]any{expected, "is not", got}, args...)...)
	}
}

func True(tb testing.TB, cond bool, args ...any) {
	tb.Helper()
	if !cond {
		tb.Fatal(append([]any{"is false"}, args...)...)
	}
}
