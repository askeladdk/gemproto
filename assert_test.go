package gemproto_test

import (
	"reflect"
	"testing"
)

func requireNoError(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func assertEqual(t *testing.T, expected, got any) {
	if !reflect.DeepEqual(expected, got) {
		t.Errorf("%s != %s", expected, got)
	}
}
