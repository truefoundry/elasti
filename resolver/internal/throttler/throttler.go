package throttler

import (
	"context"
	"fmt"
	"time"

	"github.com/truefoundry/elasti/pkg/k8sHelper"
	"github.com/truefoundry/elasti/pkg/messages"
	"go.uber.org/zap"
)

type (
	Throttler struct {
		logger        *zap.Logger
		breaker       *Breaker
		k8sUtil       *k8sHelper.Ops
		retryDuration time.Duration
	}

	ThrottlerParams struct {
		QueueRetryDuration time.Duration
		K8sUtil            *k8sHelper.Ops
		QueueDepth         int
		MaxConcurrency     int
		InitialCapacity    int
		Logger             *zap.Logger
	}
)

func NewThrottler(param *ThrottlerParams) *Throttler {
	breaker := NewBreaker(BreakerParams{
		QueueDepth:      param.QueueDepth,
		MaxConcurrency:  param.MaxConcurrency,
		InitialCapacity: param.InitialCapacity,
		Logger:          param.Logger,
	})

	return &Throttler{
		logger:        param.Logger.With(zap.String("component", "throttler")),
		breaker:       breaker,
		k8sUtil:       param.K8sUtil,
		retryDuration: param.QueueRetryDuration,
	}
}

func (t *Throttler) Try(ctx context.Context, host *messages.Host, resolve func(int) error) error {
	reenqueue := true
	tryCount := 1
	var tryErr error

	for reenqueue {
		breakErr := t.breaker.Maybe(ctx, func() {
			if isPodActive, err := t.k8sUtil.CheckIfServiceEnpointActive(host.Namespace, host.TargetService); err != nil {
				tryErr = fmt.Errorf("unable to get target active endpoints: %w", err)
			} else if !isPodActive {
				tryErr = fmt.Errorf("no active endpoints found for namespace: %v service: %v", host.Namespace, host.TargetService)
			} else if res := resolve(tryCount); res != nil {
				tryErr = fmt.Errorf("resolve error: %w", res)
				reenqueue = false
			}

			select {
			case <-ctx.Done():
				// NOTE: We have commited it to stop it from overridding the previous error
				//tryErr = fmt.Errorf("context done error: %w", ctx.Err())
				reenqueue = false
			default:
				if reenqueue {
					tryCount++
					time.Sleep(t.retryDuration)
				}
			}
		})
		if breakErr != nil {
			return fmt.Errorf("breaker error: %w", breakErr)
		}
	}
	if tryErr != nil {
		return fmt.Errorf("thunk error: %w retry count: %v", tryErr, tryCount)
	}
	return nil
}
