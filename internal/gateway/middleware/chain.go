package middleware

import (
	"bytes"
	"net/http"
)

// Middleware is a function that wraps an HTTP handler.
type Middleware func(http.Handler) http.Handler

// Chain represents a composable chain of middleware.
// Middleware can be added to the chain and then applied to a handler.
type Chain struct {
	middlewares []Middleware
}

// NewChain creates a new middleware chain.
func NewChain(middlewares ...Middleware) Chain {
	return Chain{
		middlewares: append([]Middleware{}, middlewares...),
	}
}

// Append adds middleware to the end of the chain.
// Returns a new chain with the additional middleware.
func (c Chain) Append(middlewares ...Middleware) Chain {
	newMiddlewares := make([]Middleware, len(c.middlewares)+len(middlewares))
	copy(newMiddlewares, c.middlewares)
	copy(newMiddlewares[len(c.middlewares):], middlewares)
	return Chain{middlewares: newMiddlewares}
}

// Then applies the chain to a handler.
// Middleware is applied in order: first middleware is the outermost wrapper.
func (c Chain) Then(h http.Handler) http.Handler {
	if h == nil {
		h = http.DefaultServeMux
	}

	// Apply middleware in reverse order so that the first middleware
	// in the chain is the outermost wrapper
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		h = c.middlewares[i](h)
	}

	return h
}

// ThenFunc applies the chain to a handler function.
func (c Chain) ThenFunc(fn http.HandlerFunc) http.Handler {
	return c.Then(fn)
}

// Len returns the number of middleware in the chain.
func (c Chain) Len() int {
	return len(c.middlewares)
}

// Recovery returns a middleware that recovers from panics.
// If a panic occurs, it logs the error and returns a 500 response.
func Recovery(logger Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Log the panic with stack trace
					logger.Error(r.Context(), "panic recovered",
						"error", err,
						"path", r.URL.Path,
						"method", r.Method,
					)

					// Return 500 error
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"error":"Internal Server Error"}`))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// Logger is the interface for logging used by middleware.
type Logger interface {
	Error(ctx interface{}, msg string, keysAndValues ...interface{})
}

// ResponseCapture is a wrapper around http.ResponseWriter that captures
// the status code and response body for post-processing.
type ResponseCapture struct {
	http.ResponseWriter
	StatusCode int
	Body       *bytes.Buffer
	wroteHeader bool
}

// NewResponseCapture creates a new ResponseCapture.
func NewResponseCapture(w http.ResponseWriter) *ResponseCapture {
	return &ResponseCapture{
		ResponseWriter: w,
		StatusCode:     http.StatusOK,
		Body:           &bytes.Buffer{},
	}
}

// WriteHeader captures the status code and delegates to the underlying writer.
func (rc *ResponseCapture) WriteHeader(code int) {
	if !rc.wroteHeader {
		rc.StatusCode = code
		rc.wroteHeader = true
		rc.ResponseWriter.WriteHeader(code)
	}
}

// Write captures the body and delegates to the underlying writer.
func (rc *ResponseCapture) Write(b []byte) (int, error) {
	if !rc.wroteHeader {
		rc.WriteHeader(http.StatusOK)
	}
	rc.Body.Write(b)
	return rc.ResponseWriter.Write(b)
}

// Header returns the header map.
func (rc *ResponseCapture) Header() http.Header {
	return rc.ResponseWriter.Header()
}

// Written returns true if the header has been written.
func (rc *ResponseCapture) Written() bool {
	return rc.wroteHeader
}

// Capture returns a middleware that captures the response for post-processing.
// The captured response is passed to the process function after the handler completes.
func Capture(process func(*ResponseCapture)) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capture := NewResponseCapture(w)
			next.ServeHTTP(capture, r)
			process(capture)
		})
	}
}

// Skip is a middleware that conditionally skips the next middleware.
// If the condition returns true, the next handler is called directly.
func Skip(condition func(*http.Request) bool, middleware Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		handler := middleware(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if condition(r) {
				next.ServeHTTP(w, r)
				return
			}
			handler.ServeHTTP(w, r)
		})
	}
}

// If is a middleware that conditionally applies middleware.
// If the condition returns true, the middleware is applied; otherwise it's skipped.
func If(condition func(*http.Request) bool, middleware Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		handler := middleware(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if condition(r) {
				handler.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Combine combines multiple middleware into a single middleware.
// The middleware are applied in order.
func Combine(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		return NewChain(middlewares...).Then(next)
	}
}

// NoOp is a no-op middleware that does nothing.
func NoOp(next http.Handler) http.Handler {
	return next
}

// Wrap converts a standard middleware function to our Middleware type.
func Wrap(fn func(http.Handler) http.Handler) Middleware {
	return Middleware(fn)
}
