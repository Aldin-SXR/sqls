package database

import "testing"

func TestOracleRegisteredByDefault(t *testing.T) {
	if !Registered("oracle") {
		t.Fatal("standard builds must include Oracle support")
	}
}
