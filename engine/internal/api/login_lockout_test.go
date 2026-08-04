package api

import "testing"

func TestLockoutKeysAreNamespaced(t *testing.T) {
	t.Parallel()

	// A username equal to an email address must not share a lockout row with
	// the email login flow: locking one path would otherwise lock the other,
	// and a successful login on one would clear the other's failure count.
	const same = "someone@example.com"
	if emailLockoutKey(same) == passwordLockoutKey(same) {
		t.Fatal("email and password lockout keys collided")
	}
	if emailLockoutKey("a") == emailLockoutKey("b") {
		t.Fatal("distinct emails shared a lockout key")
	}
}
