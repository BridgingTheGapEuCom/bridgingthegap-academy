package platform

import (
	"math"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"
)

// These single-instance defaults admit a six-attempt burst per remote peer,
// then three attempts per minute. The map has a hard cap; full state rejects
// new peers instead of evicting active buckets and resetting their allowance.
const (
	loginBurst           = 6
	loginRefillInterval  = 20 * time.Second
	loginMaxSources      = 4096
	loginSourceIdle      = 15 * time.Minute
	loginCleanupInterval = time.Minute
	// Two default 64 MiB verifications use ~128 MiB of Argon2 working memory.
	// Two stored PHC hashes at the allowed 128 MiB maximum can use ~256 MiB.
	maxConcurrentLogins = 2
)

type loginRatePolicy struct {
	burst           int
	refillInterval  time.Duration
	maxSources      int
	idle            time.Duration
	cleanupInterval time.Duration
}

var defaultLoginRatePolicy = loginRatePolicy{
	burst: loginBurst, refillInterval: loginRefillInterval, maxSources: loginMaxSources,
	idle: loginSourceIdle, cleanupInterval: loginCleanupInterval,
}

type loginBucket struct {
	tokens   float64
	updated  time.Time
	lastSeen time.Time
}

type loginSourceLimiter struct {
	mu          sync.Mutex
	buckets     map[netip.Addr]loginBucket
	policy      loginRatePolicy
	now         func() time.Time
	lastCleanup time.Time
}

func newLoginSourceLimiter(policy loginRatePolicy, now func() time.Time) *loginSourceLimiter {
	if now == nil {
		now = time.Now
	}
	return &loginSourceLimiter{buckets: make(map[netip.Addr]loginBucket), policy: policy, now: now}
}

// Only the TCP peer is used. Forwarded-IP headers are deliberately ignored.
// IPv4-mapped IPv6 addresses share the IPv4 bucket; IPv6 addresses otherwise
// use their full address, not a privacy-invasive identity approximation.
func loginRemotePeer(remoteAddr string) (netip.Addr, bool) {
	host, portText, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return netip.Addr{}, false
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return netip.Addr{}, false
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return address.Unmap().WithZone(""), true
}

// Allow consumes the same source budget for all supplied emails and outcomes.
// The returned delay is only the coarse time until a source bucket refills.
func (l *loginSourceLimiter) Allow(remoteAddr string) (bool, time.Duration) {
	address, valid := loginRemotePeer(remoteAddr)
	if !valid {
		return false, 0
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.lastCleanup.IsZero() || now.Sub(l.lastCleanup) >= l.policy.cleanupInterval {
		for key, bucket := range l.buckets {
			if now.Sub(bucket.lastSeen) >= l.policy.idle {
				delete(l.buckets, key)
			}
		}
		l.lastCleanup = now
	}
	bucket, found := l.buckets[address]
	if !found {
		if len(l.buckets) >= l.policy.maxSources {
			return false, 0
		}
		bucket = loginBucket{tokens: float64(l.policy.burst), updated: now}
	}
	if elapsed := now.Sub(bucket.updated); elapsed > 0 {
		bucket.tokens = math.Min(float64(l.policy.burst), bucket.tokens+float64(elapsed)/float64(l.policy.refillInterval))
		bucket.updated = now
	}
	bucket.lastSeen = now
	if bucket.tokens < 1 {
		l.buckets[address] = bucket
		return false, time.Duration(math.Ceil((1 - bucket.tokens) * float64(l.policy.refillInterval)))
	}
	bucket.tokens--
	l.buckets[address] = bucket
	return true, 0
}

// A buffered channel bounds both active work and waiting: excess requests
// fail immediately rather than accumulating goroutines around Argon2id.
type loginWorkGuard struct{ permits chan struct{} }

func newLoginWorkGuard(limit int) *loginWorkGuard {
	return &loginWorkGuard{permits: make(chan struct{}, limit)}
}

func (g *loginWorkGuard) TryAcquire() bool {
	select {
	case g.permits <- struct{}{}:
		return true
	default:
		return false
	}
}

func (g *loginWorkGuard) Release() { <-g.permits }
