package k8sHelper

import (
	"context"
	"fmt"

	"github.com/truefoundry/elasti/pkg/values"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Ops help you do various operations in your kubernetes cluster
type Ops struct {
	kClient *kubernetes.Clientset
	logger  *zap.Logger
}

// NewOps create a new instance for the K9s Operations
func NewOps(logger *zap.Logger, kClient *kubernetes.Clientset) *Ops {
	return &Ops{
		logger:  logger.Named("k8sOps"),
		kClient: kClient,
	}
}

// CheckIfServicePodActive returns true if even a single pod for a service is active
func (k *Ops) CheckIfServicePodActive(ns, svc string) (bool, error) {
	selectors, err := k.getServiceSelectorStr(ns, svc)
	if err != nil {
		return false, err
	}
	pods, err := k.kClient.CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selectors,
	})
	if err != nil {
		return false, err
	}
	if len(pods.Items) == 0 {
		return false, ErrNoPodFound
	}
	podActive := false
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			podActive = true
			break
		}
	}
	if !podActive {
		return podActive, ErrNoActivePodFound
	}

	return podActive, nil
}

// GetServiceSelectorStr is to generate a k8s acceptable selector string for the provided service
func (k *Ops) getServiceSelectorStr(ns, svc string) (string, error) {
	service, err := k.kClient.CoreV1().Services(ns).Get(context.TODO(), svc, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	selectorString := ""
	for key, value := range service.Spec.Selector {
		if selectorString != "" {
			selectorString += ","
		}
		selectorString += fmt.Sprintf("%s=%s", key, value)
	}

	return selectorString, nil
}

// ScaleTargetWhenAtZero scales the TargetRef to the provided replicas when it's at 0
func (k *Ops) ScaleTargetWhenAtZero(ns, targetName, targetKind string, replicas int32) error {
	switch targetKind {
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
	k.logger.Debug("Scaling Rollout yet to be implimented", zap.String("rollout", targetName), zap.Int32("replicas", replicas))
	return nil
}
