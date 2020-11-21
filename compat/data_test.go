package compat

import (
	"testing"
)

func TestVersionsSorted(t *testing.T) {
	versions := APIVersions()

	if versions[0] != "6.0" {
		t.Fail()
	}
}
