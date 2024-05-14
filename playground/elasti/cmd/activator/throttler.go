package main

import (
	"context"
	"fmt"
	"regexp"

	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type breaker interface {
	Reserve(ctx context.Context) (func(), bool)
}

type Throttler struct {
	logger       *zap.Logger
	breaker      *Breaker
	k8sClientSet *kubernetes.Clientset
}

func NewThrottler(ctx context.Context, logger *zap.Logger) *Throttler {
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Debug("Not able to connect via ClusterConfig, trying via kubeconfig", zap.Error(err))
		// TODOs, make it dynamic, it will be used for local testing
		kubeconfig := "/Users/ramantehlan/.kube/config"
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			logger.Fatal("Not able to connect to cluster config", zap.Error(err))
		}
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatal("Error connecting with kubernetes", zap.Error(err))
	}
	logger.Debug("Connected to kubernetes", zap.Any("clientSet", clientset))
	return &Throttler{
		logger: logger,
		// TODOs: We will make this parameter dynamic
		breaker:      NewBreaker(100),
		k8sClientSet: clientset,
	}
}

func (t *Throttler) Try(ctx context.Context, ns, svc string, resolve func() error) error {
	pods, err := t.getActivePods(ns, svc)
	if err != nil {
		t.logger.Error("Error getting pods", zap.Error(err))
		return err
	}
	t.logger.Debug("pods", zap.Any("pods", pods))

	reenqueue := true
	for reenqueue {
		reenqueue = false
		if err := resolve(); err != nil {
			t.logger.Info("Error resolving request", zap.Error(err))
		}
	}
	return nil
}

func (t *Throttler) getActivePods(ns, svc string) ([]string, error) {
	selectors, err := t.getServiceSelector(ns, svc)
	if err != nil {
		t.logger.Fatal("Error getting pod selectors", zap.Error(err))
	}
	t.logger.Debug("selectors", zap.Any("selectors", selectors))

	// TODOs: We might need to check if the selector has a value for app or not
	// Here we are assuming that the value is present

	pods, err := t.k8sClientSet.CoreV1().Pods(ns).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", selectors["app"]),
	})
	if err != nil {
		return nil, err
	}

	activePods := []string{}
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			activePods = append(activePods, pod.Name)
		}
	}

	return activePods, nil
}

func (t *Throttler) extractNamespaceAndService(s string, internal bool) (string, string, error) {
	re := regexp.MustCompile(`http://(?P<service>[^.]+)\.(?P<namespace>[^.]+)\.svc\.cluster\.local:\d+`)
	// When the request come internal source, we don't get a http
	if internal {
		re = regexp.MustCompile(`(?P<service>[^.]+)\.(?P<namespace>[^.]+)\.svc\.cluster\.local:\d+`)
	}
	matches := re.FindStringSubmatch(s)
	if len(matches) < 3 {
		return "", "", fmt.Errorf("unable to extract namespace and service name")
	}
	service := matches[re.SubexpIndex("service")]
	namespace := matches[re.SubexpIndex("namespace")]
	return namespace, service, nil
}

func (t *Throttler) getServiceSelector(ns, svc string) (map[string]string, error) {
	service, err := t.k8sClientSet.CoreV1().Services(ns).Get(context.TODO(), svc, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return service.Spec.Selector, nil
}
