package throttler

import (
	"context"
	"errors"
	"sync/atomic"

	"go.uber.org/zap"
)

type BreakerParams struct {
	QueueDepth      int
	MaxConcurrency  int
	InitialCapacity int
	Logger          *zap.Logger
}

// Breaker enforces a concurrency limit on the execution of a function.
// Function call attempts beyond the limit of the max-concurrency are failed immediately.
type Breaker struct {
	logger         *zap.Logger
	inFlight       atomic.Int64
	totalSlots     int64
	maxConcurrency uint16
	sem            *semaphore
}

func NewBreaker(params BreakerParams) *Breaker {
	return &Breaker{
		maxConcurrency: uint16(params.MaxConcurrency), //nolint: gosec
		totalSlots:     int64(params.QueueDepth + params.MaxConcurrency),
		logger:         params.Logger,
		sem:            newSemaphore(params.MaxConcurrency, params.InitialCapacity),
	}
}

var ErrRequestQueueFull = errors.New("request queue is full! This request is dropped")

// Maybe conditionally executes thunk based on the Breaker concurrency
// and queue parameters.
func (b *Breaker) Maybe(ctx context.Context, thunk func()) error {
	// We want to have a queue of requests
	// and a limited number of concurrent of requests taken from that queue

	if !b.tryAcquireInFlightSlot() {
		return ErrRequestQueueFull
	}

	defer b.releaseInFlightSlot()

	if err := b.sem.acquire(ctx); err != nil {
		return err
	}

	// Defer releasing capacity in the active.
	// It's safe to ignore the error returned by release since we
	// make sure the semaphore is only manipulated here and acquire
	// + release calls are equally paired.
	defer b.sem.release()

	thunk()
	return nil
}

func (b *Breaker) tryAcquireInFlightSlot() bool {
	// We can't just use an atomic increment as we need to check if we're
	// "allowed" to increment first. Since a Load and a CompareAndSwap are
	// not done atomically, we need to retry until the CompareAndSwap succeeds
	// (it fails if we're raced to it) or if we don't fulfill the condition
	// anymore.
	for {
		cur := b.inFlight.Load()
		if cur >= b.totalSlots {
			b.logger.Debug("Request above the total slots", zap.Any("current", cur), zap.Any("total slots", b.totalSlots))
			return false
		}
		if b.inFlight.CompareAndSwap(cur, cur+1) {
			b.logger.Debug("inFlight", zap.Any("current load", cur), zap.Any("current load increase", cur+1))
			return true
		}
	}
}

func (b *Breaker) releaseInFlightSlot() {
	for {
		cur := b.inFlight.Load()
		b.inFlight.CompareAndSwap(cur, cur-1)
		return
	}
}
