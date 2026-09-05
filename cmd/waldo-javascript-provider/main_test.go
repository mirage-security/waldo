package main

import (
	"reflect"
	"testing"
)

func TestEffectiveExcludes(t *testing.T) {
	if got := effectiveExcludes(nil); !reflect.DeepEqual(got, defaultExcludes) {
		t.Fatalf("got default exclusions %#v, want %#v", got, defaultExcludes)
	}
	explicit := values{"generated/**"}
	if got := effectiveExcludes(explicit); !reflect.DeepEqual(got, []string{"generated/**"}) {
		t.Fatalf("got explicit exclusions %#v", got)
	}
}
