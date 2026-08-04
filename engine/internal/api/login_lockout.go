package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	// maxPasswordLoginAttempts wrong passwords lock the username out for
	// passwordLoginLockoutTTL. Unlike the email code flow — where each code
	// carries its own attempt counter — password login has no per-attempt
	// document, so the count lives on the lockout row itself.
	maxPasswordLoginAttempts = 10
	passwordLoginLockoutTTL  = time.Hour
)

// loginLockout throttles a single login identity. It is deliberately its own
// collection rather than a counter on the credential being guessed: the email
// flow deletes prior unused codes whenever a fresh one is requested, so a
// per-code counter alone cannot stop someone from asking for a new code every
// ten guesses. The lockout row survives that.
//
// Key namespaces the identity ("email:someone@example.com", "user:admin") so
// the two login paths cannot collide or unlock each other.
type loginLockout struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	Key         string        `bson:"key"`
	Attempts    int           `bson:"attempts"`
	LockedUntil time.Time     `bson:"locked_until"`
}

func emailLockoutKey(email string) string { return "email:" + email }

func passwordLockoutKey(username string) string { return "user:" + username }

// loginLocked returns the lockout's expiry if key is currently locked out, or
// the zero Time if it is not. It checks the expiry itself rather than trusting
// that a matching document means an active lockout, since the TTL index that
// removes expired lockouts runs on a background sweep and is not instantaneous.
func (s *Server) loginLocked(ctx context.Context, key string) (time.Time, error) {
	var lockout loginLockout
	err := s.loginLockouts.FindOne(ctx, bson.M{"key": key}).Decode(&lockout)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	if !time.Now().UTC().Before(lockout.LockedUntil) {
		return time.Time{}, nil
	}
	return lockout.LockedUntil, nil
}

// lockLogin locks key out until now+ttl. Upserted (rather than inserted) so a
// repeat offender within the same window simply extends it instead of erroring
// on the unique key index.
func (s *Server) lockLogin(ctx context.Context, key string, now time.Time, ttl time.Duration) error {
	_, err := s.loginLockouts.UpdateOne(
		ctx,
		bson.M{"key": key},
		bson.M{
			"$set": bson.M{"locked_until": now.Add(ttl)},
			"$setOnInsert": bson.M{
				"_id": bson.NewObjectID(),
				"key": key,
			},
		},
		options.UpdateOne().SetUpsert(true),
	)
	return err
}

// registerFailedLogin counts one failure against key and locks it once the
// count reaches max. FindOneAndUpdate (not UpdateOne) so the post-increment
// count comes back atomically: two concurrent wrong guesses must not both read
// a stale count and both miss crossing the threshold.
//
// locked_until is seeded in the past on insert so the TTL index still reaps
// rows for identities that never reach the threshold, and so loginLocked reads
// them as unlocked in the meantime.
func (s *Server) registerFailedLogin(ctx context.Context, key string, now time.Time, max int, ttl time.Duration) error {
	var lockout loginLockout
	err := s.loginLockouts.FindOneAndUpdate(
		ctx,
		bson.M{"key": key},
		bson.M{
			"$inc": bson.M{"attempts": 1},
			"$setOnInsert": bson.M{
				"_id":          bson.NewObjectID(),
				"key":          key,
				"locked_until": now.Add(-time.Second),
			},
		},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	).Decode(&lockout)
	if err != nil {
		return err
	}
	if lockout.Attempts < max {
		return nil
	}

	return s.lockLogin(ctx, key, now, ttl)
}

// clearLoginFailures drops the failure count after a successful login so a user
// who mistypes a few times and then gets in does not carry the count forward.
func (s *Server) clearLoginFailures(ctx context.Context, key string) error {
	_, err := s.loginLockouts.DeleteOne(ctx, bson.M{"key": key})
	return err
}

// lockoutRetryMinutes formats the remaining lockout duration for the error
// message. Rounded to the nearest minute, and never below one, so the message
// never claims "try again in 0 minutes" while still locked.
func lockoutRetryMinutes(lockedUntil, now time.Time) int {
	minutes := int(lockedUntil.Sub(now).Round(time.Minute).Minutes())
	if minutes < 1 {
		return 1
	}
	return minutes
}

func writeLockoutError(w http.ResponseWriter, lockedUntil time.Time, what string) {
	minutes := lockoutRetryMinutes(lockedUntil, time.Now().UTC())
	writeError(w, http.StatusTooManyRequests, fmt.Sprintf(
		"too many incorrect %s; try again in %d minute(s)", what, minutes,
	))
}
