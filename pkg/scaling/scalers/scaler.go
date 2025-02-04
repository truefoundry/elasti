package scalers

import (
	"context"
)

type Scaler interface {
	ShouldScaleToZero(ctx context.Context) (bool, error)
	Close(ctx context.Context) error
}
