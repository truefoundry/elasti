package values

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	ArgoPhaseHealthy              = "Healthy"
	DeploymentConditionStatusTrue = "True"

	KindDeployments = "deployments"
	KindRollout     = "rollouts"
	KindService     = "services"

	ServeMode = "serve"
	ProxyMode = "proxy"
	NullMode  = ""

	Success = "success"

	DefaultCooldownPeriod = time.Second * 300
)

var (
	RolloutGVR = schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "rollouts",
	}

	ServiceGVR = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	}

	ElastiServiceGVR = schema.GroupVersionResource{
		Group:    "elasti.truefoundry.com",
		Version:  "v1alpha1",
		Resource: "elastiservices",
	}

	ScaledObjectGVR = schema.GroupVersionResource{
		Group:    "keda.sh",
		Version:  "v1alpha1",
		Resource: "scaledobjects",
	}
)
