package k8shelper

import (
	"context"
	"fmt"
	"strings"

	"github.com/truefoundry/elasti/pkg/values"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Ops help you do various operations in your kubernetes cluster
type Ops struct {
	kClient        *kubernetes.Clientset
	kDynamicClient *dynamic.DynamicClient
	logger         *zap.Logger
}

// NewOps create a new instance for the K9s Operations
func NewOps(logger *zap.Logger, config *rest.Config) *Ops {
	kClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatal("Error connecting with kubernetes", zap.Error(err))
	}
	kDynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		logger.Fatal("Error connecting with kubernetes", zap.Error(err))
	}
	return &Ops{
		logger:         logger.Named("k8sOps"),
		kClient:        kClient,
		kDynamicClient: kDynamicClient,
	}
}

// CheckIfServiceEnpointActive returns true if endpoint for a service is active
func (k *Ops) CheckIfServiceEnpointActive(ns, svc string) (bool, error) {
	endpoint, err := k.kClient.CoreV1().Endpoints(ns).Get(context.TODO(), svc, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("CheckIfServiceEnpointActive - GET: %w", err)
	}

	if len(endpoint.Subsets) > 0 {
		if len(endpoint.Subsets[0].Addresses) != 0 {
			k.logger.Debug("Service endpoint is active", zap.String("service", svc), zap.String("namespace", ns))
			return true, nil
		}
	}
	return false, nil
}

// ScaleTargetWhenAtZero scales the TargetRef to the provided replicas when it's at 0
func (k *Ops) ScaleTargetWhenAtZero(ns, targetName, targetKind string, replicas int32) error {
	k.logger.Info("Initiating scale of ScaleTargetRef", zap.String("kind", targetKind), zap.String("kind_name", targetName), zap.Int32("replicas", replicas))
	switch strings.ToLower(targetKind) {
	case values.KindDeployments:
		err := k.ScaleDeployment(ns, targetName, replicas)
		if err != nil {
			return fmt.Errorf("ScaleTargetWhenAtZero - Deployment: %w", err)
		}
	case values.KindRollout:
		err := k.ScaleArgoRollout(ns, targetName, replicas)
		if err != nil {
			return fmt.Errorf("ScaleTargetWhenAtZero - Rollout: %w", err)
		}
	default:
		return fmt.Errorf("unsupported target kind: %s", targetKind)
	}
	return nil
}

func (k *Ops) ScaleTargetToZero(namespacedName types.NamespacedName, targetKind string) error {
	switch strings.ToLower(targetKind) {
	case values.KindDeployments:
		err := k.ScaleDeployment(namespacedName.Namespace, namespacedName.Name, 0)
		if err != nil {
			return fmt.Errorf("ScaleTargetToZero - Deployment: %w", err)
		}
	case values.KindRollout:
		err := k.ScaleArgoRollout(namespacedName.Namespace, namespacedName.Name, 0)
		if err != nil {
			return fmt.Errorf("ScaleTargetToZero - Rollout: %w", err)
		}
	default:
		return fmt.Errorf("unsupported target kind: %s", targetKind)
	}
	return nil
}

// ScaleDeployment scales the deployment to the provided replicas
func (k *Ops) ScaleDeployment(ns, targetName string, replicas int32) error {
	deploymentClient := k.kClient.AppsV1().Deployments(ns)
	deploy, err := deploymentClient.Get(context.TODO(), targetName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("ScaleDeployment - GET: %w", err)
	}

	k.logger.Debug("Deployment found", zap.String("deployment", targetName), zap.Int32("current replicas", *deploy.Spec.Replicas), zap.Int32("desired replicas", replicas))
	if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas != replicas {
		deploy.Spec.Replicas = &replicas
		_, err = deploymentClient.Update(context.TODO(), deploy, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("ScaleDeployment - Update: %w", err)
		}
		k.logger.Info("Deployment scaled", zap.String("deployment", targetName), zap.Int32("replicas", replicas))
		return nil
	}
	k.logger.Info("Deployment already scaled", zap.String("deployment", targetName), zap.Int32("current replicas", *deploy.Spec.Replicas))
	return nil
}

// ScaleArgoRollout scales the rollout to the provided replicas
func (k *Ops) ScaleArgoRollout(ns, targetName string, replicas int32) error {
	rollout, err := k.kDynamicClient.Resource(values.RolloutGVR).Namespace(ns).Get(context.TODO(), targetName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("ScaleArgoRollout - GET: %w", err)
	}

	currentReplicas := rollout.Object["spec"].(map[string]interface{})["replicas"].(int64)
	k.logger.Info("Rollout found", zap.String("rollout", targetName), zap.Int64("current replicas", currentReplicas), zap.Int32("desired replicas", replicas))

	if currentReplicas != int64(replicas) {
		rollout.Object["spec"].(map[string]interface{})["replicas"] = replicas
		_, err = k.kDynamicClient.Resource(values.RolloutGVR).Namespace(ns).Update(context.TODO(), rollout, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("ScaleArgoRollout - Update: %w", err)
		}
		k.logger.Info("Rollout scaled", zap.String("rollout", targetName), zap.Int32("replicas", replicas))
		return nil
	}
	k.logger.Info("Rollout already scaled", zap.String("rollout", targetName), zap.Int64("current replicas", currentReplicas))

	return nil
}
