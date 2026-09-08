package messaging

import (
	"context"
	"time"
)

// SetSleepForTest replaces the consumer's backoff sleep.
//
// It lives in an _test.go file so it exists only while the package's own tests
// are compiled: the production API is unchanged, and no other package can
// reach it. The alternative — exporting a Clock or a Sleeper on
// ConsumerConfig — would put a field in the public surface whose only purpose
// is to be replaced by a test, and every caller would have to decide what to
// pass for it.
func SetSleepForTest(c *Consumer, sleep func(ctx context.Context, d time.Duration) error) {
	c.sleep = sleep
}
