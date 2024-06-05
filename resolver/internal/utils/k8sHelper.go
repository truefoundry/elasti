package utils

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type K8sHelper struct {
	client *kubernetes.Clientset
	logger *zap.Logger
}

func NewK8sUtil(logger *zap.Logger) *K8sHelper {
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatal("Error fetching cluster config", zap.Error(err))
	}
	clientset, cerr := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatal("Error connecting with kubernetes", zap.Error(cerr))
	}

	return &K8sHelper{
		logger: logger,
		client: clientset,
	}
}

func (k *K8sHelper) CheckIfPodsActive(ns, svc string) (bool, error) {
	selectors, err := k.GetServiceSelectorStr(ns, svc)
	if err != nil {
		k.logger.Error("Error getting pod selectors", zap.Error(err))
		return false, err
	}
	pods, err := k.client.CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selectors,
	})
	if err != nil {
		return false, err
	}
	if len(pods.Items) == 0 {
		return false, fmt.Errorf("no pods found for service %s", svc)
	}
	podActive := false
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			podActive = true
			break
		}
	}
	return podActive, nil
}

func (k *K8sHelper) GetServiceSelectorStr(ns, svc string) (string, error) {
	service, err := k.client.CoreV1().Services(ns).Get(context.TODO(), svc, metav1.GetOptions{})
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
