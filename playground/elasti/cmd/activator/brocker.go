package main

import (
	"context"
	"sync/atomic"
)

type Breaker struct {
	inFlight   atomic.Int64
	totalSlots int64
}

func NewBreaker(totalSlots int64) *Breaker {
	return &Breaker{
		totalSlots: totalSlots,
	}
}

func (b *Breaker) Reserve(ctx context.Context) {
	// Try to acquire the pending transaction,
	// if not found, return false
	// Else return the callback and true.

}
