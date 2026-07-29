package buildinfo

import "testing"

func TestCurrentUsesBuildVariables(t *testing.T) {
	if Current() != (Info{Version: Version, Revision: Revision}) {
		t.Fatalf("Current() = %#v", Current())
	}
}
