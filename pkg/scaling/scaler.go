package scaling

import (
	"context"
	"truefoundry/elasti/operator/api/v1alpha1"
)

type Scaler interface {
	ShouldScaleToZero(ctx context.Context, es *v1alpha1.ElastiService) (bool, error)
	Close(ctx context.Context) error
}
