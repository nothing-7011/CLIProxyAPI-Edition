package ratelimit

import (
	"sync"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/registry"
	log "github.com/sirupsen/logrus"
)

type tokenBucket struct {
	tokens     float64
	lastUpdate time.Time
	capacity   float64
	rate       float64
}

// RateLimiter manages request rate limiting based on API key and model.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket // key: apiKey + "|" + model
	rules   map[string]int          // key: apiKey + "|" + model -> RPH
}

// NewRateLimiter creates a new RateLimiter instance.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*tokenBucket),
		rules:   make(map[string]int),
	}
}

// Update validates and updates the rate limiting rules.
func (rl *RateLimiter) Update(rules []config.RateLimitRule, validAPIKeys []string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	validKeysMap := make(map[string]struct{})
	for _, k := range validAPIKeys {
		validKeysMap[k] = struct{}{}
	}

	newRules := make(map[string]int)

	for _, rule := range rules {
		if _, ok := validKeysMap[rule.APIKey]; !ok {
			log.Errorf("RateLimit: ignoring rule for unknown API key: %s", rule.APIKey)
			continue
		}
		if rule.RPH <= 0 {
			log.Errorf("RateLimit: ignoring rule with invalid RPH: %d", rule.RPH)
			continue
		}
		// Check model registry
		info := registry.GetGlobalRegistry().GetModelInfo(rule.Model)
		if info == nil {
			log.Errorf("RateLimit: ignoring rule for unknown model: %s", rule.Model)
			continue
		}

		key := rule.APIKey + "|" + rule.Model
		newRules[key] = rule.RPH
	}

	rl.rules = newRules

	// Cleanup buckets for removed rules
	for k := range rl.buckets {
		if _, ok := rl.rules[k]; !ok {
			delete(rl.buckets, k)
		}
	}
	log.Infof("RateLimit: updated with %d active rules", len(newRules))
}

// Check checks if a request is allowed. Returns true if allowed, false if limit exceeded.
func (rl *RateLimiter) Check(apiKey, model string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	key := apiKey + "|" + model
	rph, ok := rl.rules[key]
	if !ok {
		return true // No rule, allowed
	}

	bucket, exists := rl.buckets[key]
	now := time.Now()
	if !exists {
		// Initialize bucket with full capacity
		bucket = &tokenBucket{
			tokens:     float64(rph),
			capacity:   float64(rph),
			rate:       float64(rph) / 3600.0,
			lastUpdate: now,
		}
		rl.buckets[key] = bucket
	} else {
		// Update bucket parameters if rule changed
		bucket.capacity = float64(rph)
		bucket.rate = float64(rph) / 3600.0
	}

	// Refill
	delta := now.Sub(bucket.lastUpdate).Seconds()
	bucket.tokens += delta * bucket.rate
	if bucket.tokens > bucket.capacity {
		bucket.tokens = bucket.capacity
	}
	bucket.lastUpdate = now

	if bucket.tokens >= 1.0 {
		bucket.tokens -= 1.0
		return true
	}

	return false
}
