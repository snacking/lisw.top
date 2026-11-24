package database

import "testing"

func TestPasswordHash(t *testing.T) {
	a, e := HashPassword("test-only-password")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := HashPassword("test-only-password")
	if a == b || !CheckPassword(a, "test-only-password") || CheckPassword(a, "wrong") || CheckPassword("invalid", "test-only-password") {
		t.Fatal("password verification failed")
	}
}
