package ratelimit

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// bucketScript is the whole algorithm: read, refill, consume, persist.
//
// It runs as a single Lua script because the three steps must be atomic.
// Implementing the same logic with GET/SET from the client leaves a window in
// which two requests both read the same token count and both consume it, which
// under load is exactly when the limit matters.
//
// The current time is passed in rather than read with redis.call('TIME'). A
// script that reads the clock is non-deterministic, and keeping it
// deterministic means it replicates and can be reasoned about. The cost is a
// dependency on the application hosts' clocks agreeing to within a few seconds,
// which they must anyway for token expiry to work.
const bucketScript = `
local key    = KEYS[1]
local rate   = tonumber(ARGV[1])
local burst  = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])
local ttl    = tonumber(ARGV[4])

local state  = redis.call('HMGET', key, 'tokens', 'ts')
local tokens = tonumber(state[1])
local ts     = tonumber(state[2])

if tokens == nil or ts == nil then
  tokens = burst
  ts = now_ms
end

-- A clock that moved backwards must not create tokens.
local elapsed = now_ms - ts
if elapsed < 0 then elapsed = 0 end

tokens = math.min(burst, tokens + elapsed * rate / 1000.0)

local allowed = 0
if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
end

redis.call('HSET', key, 'tokens', tokens, 'ts', now_ms)
redis.call('EXPIRE', key, ttl)

-- Lua numbers are floats; Redis truncates them on return, so the fractional
-- token count is sent as a scaled integer and rescaled by the caller.
return {allowed, math.floor(tokens * 1000)}
`

// RedisLimiter enforces limits in Redis so that the quota is shared by every
// replica. An in-process limiter would give each replica its own bucket, making
// the effective limit the configured value times the replica count, and would
// reset on every deploy.
type RedisLimiter struct {
	client *goredis.Client
	script *goredis.Script
	// keyPrefix namespaces the buckets so the limiter can share a database with
	// caches and sessions without a chance of collision.
	keyPrefix string
	now       func() time.Time
}

// NewRedisLimiter builds a limiter over an existing client.
func NewRedisLimiter(client *goredis.Client) *RedisLimiter {
	return &RedisLimiter{
		client:    client,
		script:    goredis.NewScript(bucketScript),
		keyPrefix: "rl",
		now:       time.Now,
	}
}

var _ Limiter = (*RedisLimiter)(nil)

// Allow consumes one token.
//
// go-redis's Script.Run sends EVALSHA first and falls back to EVAL when the
// script is not cached, so a Redis restart or a failover to a replica that
// never saw the script costs one extra round trip instead of an error.
func (l *RedisLimiter) Allow(ctx context.Context, scope Scope, identity string, rule Rule) (Decision, error) {
	if !rule.Valid() {
		return Decision{}, fmt.Errorf("ratelimit: invalid rule for scope %q", scope)
	}

	now := l.now()
	key := fmt.Sprintf("%s:%s:%s", l.keyPrefix, scope, identity)

	res, err := l.script.Run(ctx, l.client, []string{key},
		rule.ratePerSecond(),
		rule.Burst,
		now.UnixMilli(),
		int(rule.ttl().Seconds()),
	).Int64Slice()
	if err != nil {
		return Decision{}, fmt.Errorf("ratelimit: redis: %w", err)
	}
	if len(res) != 2 {
		return Decision{}, fmt.Errorf("ratelimit: unexpected script result of length %d", len(res))
	}

	tokens := float64(res[1]) / 1000.0
	if res[0] == 1 {
		return allowDecision(rule, tokens, now), nil
	}
	return denyDecision(rule, tokens, now), nil
}
