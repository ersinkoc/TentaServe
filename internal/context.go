// Package internal provides shared types and utilities for Tentaserve.
//
// This file defines context key types and helper functions for storing
// and retrieving request-scoped values from context.Context.
package internal

import (
	"context"
	"time"
)

// contextKey is an unexported type for context keys to avoid collisions.
// Using a custom type instead of string prevents external packages from
// accidentally using the same key.
type contextKey int

const (
	// ctxKeyRequestID stores the unique request identifier.
	ctxKeyRequestID contextKey = iota

	// ctxKeyAuthResult stores the authentication result.
	ctxKeyAuthResult

	// ctxKeyUpstreamName stores the name of the upstream being called.
	ctxKeyUpstreamName

	// ctxKeyStartTime stores the request start time for latency calculation.
	ctxKeyStartTime

	// ctxKeyClientIP stores the client's IP address.
	ctxKeyClientIP

	// ctxKeyUserAgent stores the client's User-Agent string.
	ctxKeyUserAgent

	// ctxKeyCacheStatus stores the cache hit/miss status.
	ctxKeyCacheStatus
)

// RequestID returns the request ID from context, or empty string if not set.
func RequestID(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}

// WithRequestID returns a new context with the request ID set.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// AuthResult represents the result of authentication.
type AuthResult struct {
	// Authenticated is true if the request is authenticated.
	Authenticated bool

	// Subject is the authenticated entity (user ID, service name, etc.)
	Subject string

	// Claims contains any additional claims from the authentication.
	Claims map[string]any

	// Error is set if authentication failed.
	Error error
}

// IsAuthenticated returns true if the request is authenticated.
func (a *AuthResult) IsAuthenticated() bool {
	return a != nil && a.Authenticated
}

// GetClaim returns a claim value by key.
func (a *AuthResult) GetClaim(key string) (any, bool) {
	if a == nil || a.Claims == nil {
		return nil, false
	}
	v, ok := a.Claims[key]
	return v, ok
}

// AuthResultFromContext returns the auth result from context, or nil if not set.
func AuthResultFromContext(ctx context.Context) *AuthResult {
	v, _ := ctx.Value(ctxKeyAuthResult).(*AuthResult)
	return v
}

// WithAuthResult returns a new context with the auth result set.
func WithAuthResult(ctx context.Context, result *AuthResult) context.Context {
	return context.WithValue(ctx, ctxKeyAuthResult, result)
}

// UpstreamName returns the upstream name from context, or empty string if not set.
func UpstreamName(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyUpstreamName).(string)
	return v
}

// WithUpstreamName returns a new context with the upstream name set.
func WithUpstreamName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, ctxKeyUpstreamName, name)
}

// StartTime returns the request start time from context, or zero time if not set.
func StartTime(ctx context.Context) time.Time {
	v, _ := ctx.Value(ctxKeyStartTime).(time.Time)
	return v
}

// WithStartTime returns a new context with the start time set.
func WithStartTime(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, ctxKeyStartTime, t)
}

// Elapsed returns the elapsed time since the request started.
// Returns 0 if start time is not set.
func Elapsed(ctx context.Context) time.Duration {
	start := StartTime(ctx)
	if start.IsZero() {
		return 0
	}
	return time.Since(start)
}

// ClientIP returns the client IP from context, or empty string if not set.
func ClientIP(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyClientIP).(string)
	return v
}

// WithClientIP returns a new context with the client IP set.
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ctxKeyClientIP, ip)
}

// UserAgent returns the User-Agent from context, or empty string if not set.
func UserAgent(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyUserAgent).(string)
	return v
}

// WithUserAgent returns a new context with the User-Agent set.
func WithUserAgent(ctx context.Context, ua string) context.Context {
	return context.WithValue(ctx, ctxKeyUserAgent, ua)
}

// CacheStatus represents the cache status of a response.
type CacheStatus string

const (
	// CacheHit indicates the response was served from cache.
	CacheHit CacheStatus = "HIT"

	// CacheMiss indicates the response was not in cache.
	CacheMiss CacheStatus = "MISS"

	// CacheStale indicates the response was served from stale cache.
	CacheStale CacheStatus = "STALE"

	// CacheBypass indicates caching was bypassed.
	CacheBypass CacheStatus = "BYPASS"
)

// CacheStatusFromContext returns the cache status from context, or empty if not set.
func CacheStatusFromContext(ctx context.Context) CacheStatus {
	v, _ := ctx.Value(ctxKeyCacheStatus).(CacheStatus)
	return v
}

// WithCacheStatus returns a new context with the cache status set.
func WithCacheStatus(ctx context.Context, status CacheStatus) context.Context {
	return context.WithValue(ctx, ctxKeyCacheStatus, status)
}

// RequestContext creates a new context with all request-scoped values initialized.
// This should be called at the start of each request.
func RequestContext(parent context.Context, requestID, clientIP, userAgent string) context.Context {
	ctx := parent
	ctx = WithRequestID(ctx, requestID)
	ctx = WithStartTime(ctx, time.Now())
	ctx = WithClientIP(ctx, clientIP)
	ctx = WithUserAgent(ctx, userAgent)
	return ctx
}
