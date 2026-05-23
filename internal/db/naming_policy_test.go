package db

import "testing"

func TestSingleRecordName(t *testing.T) {
	if got := SingleRecordName("invoice-settings"); got != "invoice-settings" {
		t.Fatalf("SingleRecordName() = %q, want invoice-settings", got)
	}
}
