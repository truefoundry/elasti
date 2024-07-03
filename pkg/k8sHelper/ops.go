package k8sHelper

import (
	"context"
	"strings"

	"github.com/truefoundry/elasti/pkg/values"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		return false, err
	}

	if endpoint.Subsets != nil && len(endpoint.Subsets) > 0 {
		if endpoint.Subsets[0].Addresses != nil && len(endpoint.Subsets[0].Addresses) != 0 {
			k.logger.Debug("Checking if service endpoint is active", zap.String("service", svc), zap.String("namespace", ns), zap.Any("endpoint", endpoint.Subsets[0].Addresses))
			k.logger.Debug("Service endpoint is active", zap.String("service", svc), zap.String("namespace", ns))
			return true, nil
		}
	}
	k.logger.Debug("Service endpoint is not active", zap.String("service", svc), zap.String("namespace", ns), zap.Any("endpoint", endpoint.Subsets))
	return false, nil
}

// ScaleTargetWhenAtZero scales the TargetRef to the provided replicas when it's at 0
func (k *Ops) ScaleTargetWhenAtZero(ns, targetName, targetKind string, replicas int32) error {
	switch strings.ToLower(targetKind) {
	case values.KindDeployments:
		k.logger.Info("ScaleTargetRef kind is deployment", zap.String("kind", targetKind))
		err := k.ScaleDeployment(ns, targetName, replicas)
		if err != nil {
			return err
		}
	case values.KindRollout:
		k.logger.Info("ScaleTargetRef kind is rollout", zap.String("kind", targetKind))
		err := k.ScaleArgoRollout(ns, targetName, replicas)
		if err != nil {
			return err
		}
	default:
		k.logger.Error("Unsupported target kind", zap.String("kind", targetKind))
	}
	return nil
}

func (k *Ops) ScaleDeployment(ns, targetName string, replicas int32) error {
	k.logger.Debug("Scaling deployment", zap.String("deployment", targetName), zap.Int32("replicas", replicas))
	deploymentClient := k.kClient.AppsV1().Deployments(ns)
	deploy, err := deploymentClient.Get(context.TODO(), targetName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	k.logger.Debug("Deployment found", zap.String("deployment", targetName), zap.Int32("current replicas", *deploy.Spec.Replicas), zap.Int32("desired replicas", replicas))
	if *deploy.Spec.Replicas == 0 {
		deploy.Spec.Replicas = &replicas
		_, err = deploymentClient.Update(context.TODO(), deploy, metav1.UpdateOptions{})
		return err
	}
	return nil
}

func (k *Ops) ScaleArgoRollout(ns, targetName string, replicas int32) error {
	k.logger.Debug("Scaling Rollout", zap.String("rollout", targetName), zap.Int32("replicas", replicas))

	rollout, err := k.kDynamicClient.Resource(values.RolloutGVR).Namespace(ns).Get(context.TODO(), targetName, metav1.GetOptions{})
	if err != nil {
		k.logger.Error("Error getting rollout", zap.Error(err), zap.String("rollout", targetName))
		return err
	}

	currentReplicas := rollout.Object["spec"].(map[string]interface{})["replicas"].(int64)
	k.logger.Debug("Rollout found", zap.String("rollout", targetName), zap.Int64("current replicas", currentReplicas), zap.Int32("desired replicas", replicas))
	if currentReplicas == 0 {
		rollout.Object["spec"].(map[string]interface{})["replicas"] = replicas
		_, err = k.kDynamicClient.Resource(values.RolloutGVR).Namespace(ns).Update(context.TODO(), rollout, metav1.UpdateOptions{})
		if err != nil {
			k.logger.Error("Error updating rollout", zap.Error(err), zap.String("rollout", targetName))
			return err
		}
		k.logger.Info("Rollout scaled", zap.String("rollout", targetName), zap.Int32("replicas", replicas))
	}

	return nil
}
