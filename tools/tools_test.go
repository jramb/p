package tools

import (
	"testing"
)

func assert(t *testing.T, assertion bool, expectation string) {
	if !assertion {
		t.Error("Failed: " + expectation)
	}
}

func TestUUID(t *testing.T) {
	uuid := newUUID()
	assert(t, uuid != "", "uuid is not empty")
}
