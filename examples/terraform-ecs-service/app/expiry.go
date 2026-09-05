package app

import "time"

// waldo:correctness-critical-deferred-work
func ScheduleExpiryNotification(delay time.Duration, notify func()) {
	time.AfterFunc(delay, notify)
}
