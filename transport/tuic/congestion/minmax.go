package congestion

import (
	"math"
	"time"

	"golang.org/x/exp/constraints"
)

// InfDuration is a duration of infinite length
const InfDuration = time.Duration(math.MaxInt64)

func Max[T constraints.Ordered](a, b T) T {
	if a < b {
		return b
	}
	return a
}

func Min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

// MinNonZeroDuration return the minimum duration that's not zero.
func MinNonZeroDuration(a, b time.Duration) time.Duration {
	if a == 0 {
		return b
	}
	if b == 0 {
		return a
	}
	return Min(a, b)
}

// AbsDuration returns the absolute value of a time duration
func AbsDuration(d time.Duration) time.Duration {
	if d >= 0 {
		return d
	}
	return -d
}

// MinTime returns the earlier time
func MinTime(a, b time.Time) time.Time {
	if a.After(b) {
		return b
	}
	return a
}

// MinNonZeroTime returns the earlist time that is not time.Time{}
// If both a and b are time.Time{}, it returns time.Time{}
func MinNonZeroTime(a, b time.Time) time.Time {
	if a.IsZero() {
		return b
	}
	if b.IsZero() {
		return a
	}
	return MinTime(a, b)
}

// MaxTime returns the later time
func MaxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
