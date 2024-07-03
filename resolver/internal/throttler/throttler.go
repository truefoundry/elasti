package throttler

import (
	"context"
	"time"

	"github.com/truefoundry/elasti/pkg/k8sHelper"
	"github.com/truefoundry/elasti/pkg/messages"
	"go.uber.org/zap"
)

type Throttler struct {
	logger        *zap.Logger
	breaker       *Breaker
	k8sUtil       *k8sHelper.Ops
	retryDuration time.Duration
}

func NewThrottler(ctx context.Context, logger *zap.Logger, k8sUtil *k8sHelper.Ops) *Throttler {
	return &Throttler{
		logger: logger.With(zap.String("component", "throttler")),
		// TODOs: We will make this parameter dynamic
		breaker: NewBreaker(BreakerParams{
			QueueDepth:      200,
			MaxConcurrency:  10,
			InitialCapacity: 200,
			Logger:          logger,
		}),
		k8sUtil:       k8sUtil,
		retryDuration: 5 * time.Second,
	}
}

func (t *Throttler) Try(ctx context.Context, host *messages.Host, resolve func(int) error) error {
	reenqueue := true
	retryCount := 1
	for reenqueue {
		reenqueue = false
		var tryErr error
		if tryErr = t.breaker.Maybe(ctx, func() {
			if isPodActive, err := t.k8sUtil.CheckIfServiceEnpointActive(host.Namespace, host.TargetService); err != nil {
				t.logger.Info("Unable to get target active endpoints", zap.Error(err), zap.Int("retryCount", retryCount))
				reenqueue = true
			} else if !isPodActive {
				t.logger.Info("No active endpoints", zap.Any("host", host), zap.Int("retryCount", retryCount))
				reenqueue = true
			} else {
				if res := resolve(retryCount); res != nil {
					t.logger.Error("Error resolving proxy request", zap.Error(res), zap.Int("retryCount", retryCount))
					reenqueue = false
					tryErr = res
					return
				}
			}

			if reenqueue {
				retryCount++
				time.Sleep(t.retryDuration)
			}
		}); tryErr != nil {
			t.logger.Error("Error resolving request", zap.Error(tryErr), zap.Int("retryCount", retryCount))
		}
	}
	return nil
}
