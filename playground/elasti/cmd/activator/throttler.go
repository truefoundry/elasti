package main

import (
	"context"

	"go.uber.org/zap"
)

type breaker interface {
	Reserve(ctx context.Context) (func(), bool)
}

type Throttler struct {
	logger  *zap.Logger
	breaker *Breaker
}

func NewThrottler(ctx context.Context, logger *zap.Logger) *Throttler {
	return &Throttler{
		logger: logger,

		// TODOs: We will make this parameter dynamic
		breaker: NewBreaker(100),
	}
}

func (t *Throttler) Try(ctx context.Context, target string, resolve func() error) error {
	// We want to establish if the destination is up or not.
	// If the destination is not up, we will requeue the request, and
	// try again

	t.logger.Debug("Forwarding the request to resolve")
	// For now letting the request pass through
	if err := resolve(); err != nil {
		t.logger.Info("Error resolving request", zap.Error(err))
	}

	return nil
}
