package hostmanager

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/truefoundry/elasti/pkg/messages"
	"go.uber.org/zap"
)

func TestGetHost(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	hm := NewHostManager(logger, 10*time.Second, "X-Envoy-Decorator-Operation")

	tests := []struct {
		name          string
		req           *http.Request
		expectedHost  *messages.Host
		expectedError bool
	}{
		{
			name: "Host in header",
			req: &http.Request{
				Host: "target.com",
				Header: http.Header{
					"X-Envoy-Decorator-Operation": []string{"service.namespace.svc.cluster.local:8080/test/*"},
				},
			},
			expectedHost: &messages.Host{
				IncomingHost:   "service.namespace.svc.cluster.local:8080/test/*",
				Namespace:      "namespace",
				SourceService:  "service",
				TargetService:  "elasti-service-pvt-9df6b026a8",
				SourceHost:     "http://service.namespace.svc.cluster.local:8080",
				TargetHost:     "http://elasti-service-pvt-9df6b026a8.namespace.svc.cluster.local:8080",
				TrafficAllowed: true,
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, err := hm.GetHost(tt.req)
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedHost, host)
		})
	}
}
