package main

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var K8sUtil *k8sHelper

type k8sHelper struct {
	client *kubernetes.Clientset
	logger *zap.Logger
}

func InitK8sUtil(logger *zap.Logger) {
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatal("Error fetching cluster config", zap.Error(err))
	}
	clientset, cerr := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatal("Error connecting with kubernetes", zap.Error(cerr))
	}

	K8sUtil = &k8sHelper{
		logger: logger,
		client: clientset,
	}
}

func (k *k8sHelper) CheckIfPodsActive(ns, svc string) (bool, error) {
	selectors, err := k.GetServiceSelectorStr(ns, svc)
	if err != nil {
		k.logger.Error("Error getting pod selectors", zap.Error(err))
		return false, err
	}
	k.logger.Debug("selectors", zap.Any("selectors", selectors))
	pods, err := k.client.CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selectors,
	})
	if err != nil {
		return false, err
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

func (k *k8sHelper) GetServiceSelectorStr(ns, svc string) (string, error) {
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

func (k *k8sHelper) GetPodHealth(clientset *kubernetes.Clientset, namespace, selector string) (map[string]string, error) {
	podHealth := make(map[string]string)
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, err
	}
	for _, pod := range pods.Items {
		if !k.isPodReady(&pod) {
			podHealth[pod.Name] = "Not Ready"
		} else {
			podHealth[pod.Name] = "Ready"
		}
	}
	return podHealth, nil
}

func (k *k8sHelper) isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status != corev1.ConditionTrue {
			return false
		}
	}
	return true
}
