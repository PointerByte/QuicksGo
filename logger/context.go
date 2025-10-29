package logger

import (
	"context"
	"fmt"
	"time"
)

// Context is a custom context similar to gin.Context
type Context struct {
	context.Context           // hereda Cancel, Deadline, Done, Value
	startTime       time.Time // ejemplo: marca de inicio
	fields          map[any]any
}

// New creates a new logger.Context based on a parent context
func New(parent context.Context) *Context {
	newContext := &Context{
		Context:   parent,
		startTime: time.Now(),
		fields:    make(map[any]any),
	}
	setOtelIds(newContext)
	return newContext
}

// Set adds a key-value pair to the context
func (c *Context) Set(key any, value any) {
	c.fields[key] = value
}

// Get retrieves a value from the context
func (c *Context) Get(key any) (any, bool) {
	v, ok := c.fields[key]
	return v, ok
}

// MustGet retrieves a value or panics if it doesn’t exist
func (c *Context) MustGet(key string) any {
	if v, ok := c.Get(key); ok {
		return v
	}
	panic(fmt.Sprintf("logger.Context: key '%s' not found", key))
}

// GetLatency returns the elapsed time since the context was created.
// It measures how long the operation associated with this context has been running.
func (c *Context) GetLatency() time.Duration {
	return time.Since(c.startTime)
}

// WithCancel creates a copy of the logger.Context with manual cancellation support
func (c *Context) WithCancel() (*Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(c.Context)
	newCtx := &Context{
		Context:   ctx,
		startTime: c.startTime,
		fields:    c.fields,
	}
	return newCtx, cancel
}

// WithTimeout creates a copy of the logger.Context that automatically
// cancels after the given duration
func (c *Context) WithTimeout(d time.Duration) (*Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(c.Context, d)
	newCtx := &Context{
		Context:   ctx,
		startTime: c.startTime,
		fields:    c.fields,
	}
	return newCtx, cancel
}
