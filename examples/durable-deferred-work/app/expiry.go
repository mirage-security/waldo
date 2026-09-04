package app

import "time"

// waldo:correctness-critical-deferred-work
func ScheduleExpiryNotification(delay time.Duration, notify func()) {
	time.AfterFunc(delay, notify)
}

func ScheduleBestEffortTelemetry(delay time.Duration, flush func()) {
	time.AfterFunc(delay, flush)
}
