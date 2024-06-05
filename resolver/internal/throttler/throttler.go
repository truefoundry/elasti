package throttler

import (
	"context"
	"time"
	"truefoundry/resolver/internal/utils"

	"github.com/truefoundry/elasti/pkg/messages"
	"go.uber.org/zap"
)

type Throttler struct {
	logger  *zap.Logger
	breaker *Breaker
	k8sUtil *utils.K8sHelper
}

func NewThrottler(ctx context.Context, logger *zap.Logger, k8sUtil *utils.K8sHelper) *Throttler {
	return &Throttler{
		logger: logger.With(zap.String("component", "throttler")),
		// TODOs: We will make this parameter dynamic
		breaker: NewBreaker(BreakerParams{
			QueueDepth:      200,
			MaxConcurrency:  10,
			InitialCapacity: 200,
			Logger:          logger,
		}),
		k8sUtil: k8sUtil,
	}
}

func (t *Throttler) Try(ctx context.Context, host *messages.Host, resolve func(int) error) error {
	reenqueue := true
	retryCount := 1
	for reenqueue {
		reenqueue = false
		if err := t.breaker.Maybe(ctx, func() {
			if isPodActive, err := t.k8sUtil.CheckIfPodsActive(host.Namespace, host.TargetService); err != nil {
				t.logger.Info("Unable to get target active pod", zap.Error(err), zap.Int("retryCount", retryCount))
				reenqueue = true
			} else if !isPodActive {
				t.logger.Info("No active pods", zap.Any("host", host), zap.Int("retryCount", retryCount))
				reenqueue = true
			} else {
				if res := resolve(retryCount); res != nil {
					t.logger.Error("Error resolving proxy request", zap.Error(res), zap.Int("retryCount", retryCount))
					reenqueue = true
				}
			}
			if reenqueue {
				retryCount++
				time.Sleep(3 * time.Second)
			}
		}); err != nil {
			t.logger.Info("Error resolving request", zap.Error(err), zap.Int("retryCount", retryCount))
		}
	}
	return nil
}