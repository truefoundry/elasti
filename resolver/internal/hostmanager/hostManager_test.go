package hostmanager

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/truefoundry/elasti/pkg/messages"
	"github.com/truefoundry/elasti/resolver/internal/kubecache"
	"go.uber.org/zap"

	core_v1 "k8s.io/api/core/v1"
	networking_v1 "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	ServiceGVR = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	}

	IngressGVR = schema.GroupVersionResource{
		Group:    "networking.k8s.io",
		Version:  "v1",
		Resource: "ingresses",
	}
)

func TestGetHost(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	PathTypePrefix := networking_v1.PathTypePrefix

	tests := []struct {
		name          string
		req           *http.Request
		expectedHost  *messages.Host
		services      []core_v1.Service
		ingresses     []networking_v1.Ingress
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
			services: []core_v1.Service{
				{
					ObjectMeta: v1.ObjectMeta{
						Namespace: "namespace",
						Name:      "service",
					},
				},
			},
			expectedError: false,
		},
		{
			name: "Not existing service",
			req: &http.Request{
				Host: "service.namespace.svc.cluster.local",
			},
			expectedHost:  &messages.Host{},
			expectedError: true,
		},
		{
			name: "Ingress with prefix",
			req: &http.Request{
				Host: "ingress.company.com",
				URL: &url.URL{
					Path: "/prefix/with/some/extra",
				},
			},
			services: []core_v1.Service{
				{
					ObjectMeta: v1.ObjectMeta{
						Namespace: "namespace",
						Name:      "internal_service",
					},
					Spec: core_v1.ServiceSpec{
						Ports: []core_v1.ServicePort{
							{
								Name: "foo",
								Port: 8012,
							},
						},
					},
				},
			},
			ingresses: []networking_v1.Ingress{
				{
					ObjectMeta: v1.ObjectMeta{
						Namespace: "namespace",
						Name:      "public_ingress",
					},
					Spec: networking_v1.IngressSpec{
						Rules: []networking_v1.IngressRule{
							{
								Host: "ingress.company.com",
								IngressRuleValue: networking_v1.IngressRuleValue{
									HTTP: &networking_v1.HTTPIngressRuleValue{
										Paths: []networking_v1.HTTPIngressPath{
											{
												Path:     "/prefix",
												PathType: &PathTypePrefix,
												Backend: networking_v1.IngressBackend{
													Service: &networking_v1.IngressServiceBackend{
														Name: "internal_service",
														Port: networking_v1.ServiceBackendPort{
															Name: "foo",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedHost: &messages.Host{
				IncomingHost:   "ingress.company.com",
				Namespace:      "namespace",
				SourceService:  "internal_service",
				TargetService:  "elasti-internal_service-pvt-a564bb0874",
				SourceHost:     "http://ingress.company.com",
				TargetHost:     "http://elasti-internal_service-pvt-a564bb0874.namespace.svc.cluster.local:8012",
				TrafficAllowed: true,
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewClientset()
			kubeCache := kubecache.NewKubeCache(logger, clientset)

			assert.NoError(t, kubeCache.Start(context.Background()))

			hm := NewHostManager(logger, 10*time.Second, "X-Envoy-Decorator-Operation", kubeCache)

			for _, service := range tt.services {
				assert.NoError(t, clientset.Tracker().Create(ServiceGVR, &service, service.Namespace))
			}
			for _, ingress := range tt.ingresses {
				assert.NoError(t, clientset.Tracker().Create(IngressGVR, &ingress, ingress.Namespace))
			}

			time.Sleep(50 * time.Millisecond) // let goroutines process data

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
