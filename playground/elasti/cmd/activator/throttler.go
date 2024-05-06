package main

import (
	"context"

	"go.uber.org/zap"
)

type Throttler struct {
	logger    *zap.Logger
	ipAddress string
}

func NewThrottler(ctx context.Context, logger *zap.Logger, ipAddr string) *Throttler {
	return &Throttler{
		logger:    logger,
		ipAddress: ipAddr,
	}
}

func (t *Throttler) Try(ctx context.Context, resolve func(string) error) error {
	// We want to establish if the destination is up or not.
	// If the destination is not up, we will requeue the request, and
	// try again

	t.logger.Debug("Forwarding the request to resolve")
	// For now letting the request pass through
	return resolve(t.ipAddress)
}
