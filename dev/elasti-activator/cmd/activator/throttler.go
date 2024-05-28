package main

import (
	"context"

	"go.uber.org/zap"
)

type Throttler struct {
	logger  *zap.Logger
	breaker *Breaker
	k8sUtil *k8sHelper
}

func NewThrottler(ctx context.Context, logger *zap.Logger, k8sUtil *k8sHelper) *Throttler {
	return &Throttler{
		logger: logger,
		// TODOs: We will make this parameter dynamic
		breaker: NewBreaker(BreakerParams{
			QueueDepth:      5,
			MaxConcurrency:  2,
			InitialCapacity: 10,
			Logger:          logger,
		}),
		k8sUtil: k8sUtil,
	}
}

func (t *Throttler) Try(ctx context.Context, host *Host, resolve func() error) error {
	reenqueue := true
	for reenqueue {
		reenqueue = false
		if err := t.breaker.Maybe(ctx, func() {
			if isPodActive, err := t.k8sUtil.CheckIfPodsActiveV2(host.Namespace, host.TargetService); err != nil {
				t.logger.Error("Error getting pods", zap.Error(err))
				reenqueue = true
				return
			} else if !isPodActive {
				t.logger.Debug("No active pods", zap.Any("host", host))
				reenqueue = true
				return
			}
			if res := resolve(); res != nil {
				t.logger.Error("Error resolving proxy request", zap.Error(res))
				reenqueue = true
			} else {
				HostManager.DisableTrafficForHost(host.SourceService)
			}
		}); err != nil {
			t.logger.Info("Error resolving request", zap.Error(err))
		}
	}
	return nil
}
