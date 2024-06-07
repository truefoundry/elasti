package informer

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type Manager struct {
	client              *kubernetes.Clientset
	logger              *zap.Logger
	informers           sync.Map
	resyncPeriod        time.Duration
	healthCheckDuration time.Duration
}

type Info struct {
	Informer cache.SharedIndexInformer
	StopCh   chan struct{}
	Handlers cache.ResourceEventHandlerFuncs
}

func NewInformerManager(logger *zap.Logger, kConfig *rest.Config) *Manager {
	clientSet, err := kubernetes.NewForConfig(kConfig)
	if err != nil {
		logger.Fatal("Error connecting with kubernetes", zap.Error(err))
	}
	return &Manager{
		client: clientSet,
		logger: logger.Named("InformerManager"),
		// Resync period is the time after which the informer will resync the resources
		// When set to 0, it is imidiate.
		resyncPeriod:        0,
		healthCheckDuration: 5 * time.Second,
	}
}

// Start is to initiate a health check on all the running informers
// It uses HasSynced if a informer is not synced, if not, it restars it
func (m *Manager) Start() {
	m.logger.Info("Starting InformerManager")
	go wait.Until(m.monitorInformers, m.healthCheckDuration, make(<-chan struct{}))
}

func (m *Manager) monitorInformers() {
	m.informers.Range(func(key, value interface{}) bool {
		informerInfo, ok := value.(Info)
		if ok {
			if !informerInfo.Informer.HasSynced() {
				m.logger.Info("Informer not synced", zap.String("deployment_name", key.(string)))
				crdName, resourceName, namespace := parseInformerKey(key.(string))
				m.StopInformer(crdName, resourceName, namespace)
				m.enableInformer(crdName, resourceName, namespace, informerInfo.Handlers)
			}
		}
		return true
	})
}

// AddDeploymentWatch is to add a watch on a deployment
func (m *Manager) AddDeploymentWatch(crdName, deploymentName, namespace string, handlers cache.ResourceEventHandlerFuncs) {
	m.logger.Info("Adding Deployment informer", zap.String("deployment_name", deploymentName))
	m.enableInformer(crdName, deploymentName, namespace, handlers)
}

// enableInformer is to enable the informer for a deployment
func (m *Manager) enableInformer(crdName, deploymentName, namespace string, handlers cache.ResourceEventHandlerFuncs) {
	key := getInformerKey(crdName, namespace, deploymentName)
	// Proceed only if the informer is not already running, we verify by checking the map
	if _, ok := m.informers.Load(key); ok {
		m.logger.Info("Informer already running", zap.String("key", key))
		return
	}
	// Create a new shared informer
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return m.client.AppsV1().Deployments(namespace).List(context.TODO(), metav1.ListOptions{
					FieldSelector: "metadata.name=" + deploymentName,
				})
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return m.client.AppsV1().Deployments(namespace).Watch(context.TODO(), metav1.ListOptions{
					FieldSelector: "metadata.name=" + deploymentName,
				})
			},
		},
		&appsv1.Deployment{},
		m.resyncPeriod,
		cache.Indexers{},
	)
	// We pass the handlers we received as a parameter
	_, err := informer.AddEventHandler(handlers)
	if err != nil {
		m.logger.Error("Error creating informer handler", zap.Error(err))
		return
	}
	// This channel is used to stop the informer
	// We add it in the informers map, so we can stop it when required
	informerStop := make(chan struct{})
	go informer.Run(informerStop)
	// Store the informer in the map
	// This is used to manage the lifecycle of the informer
	// Recover it in case it's not syncing, this is why we also store the handlers
	// Stop it when the CRD or the operator is deleted
	m.informers.Store(key, Info{
		StopCh:   informerStop,
		Handlers: handlers,
		Informer: informer,
	})
	m.logger.Info("Informer started", zap.String("key", key))
}

// StopInformer is to stop the informer for a deployment
// It closes the shared informer for it and deletes it from the map
func (m *Manager) StopInformer(crdName, deploymentName, namespace string) {
	key := getInformerKey(crdName, deploymentName, namespace)
	m.logger.Info("Stopping Deployment watch", zap.String("key", key))
	value, ok := m.informers.Load(key)
	if !ok {
		m.logger.Info("Informer not found", zap.String("key", key))
		return
	}

	// We need to verify if the informer exists in the map
	informerInfo, ok := value.(Info)
	if !ok {
		m.logger.Error("Failed to cast WatchInfo", zap.String("key", key))
		return
	}

	// Close the informer, delete it from the map
	close(informerInfo.StopCh)
	m.informers.Delete(key)
}

// parseInformerKey is to parse the key to get the namespace and resource name
// The format of the key is crdName/resourceName/namespace
func parseInformerKey(key string) (crdName, resourceName, namespace string) {
	parts := strings.Split(key, "/")
	return parts[0], parts[1], parts[2]
}

// getInformerKey is to get the key for the informer map using namespace and resource name
func getInformerKey(crdName, resourceName, namespace string) string {
	return fmt.Sprintf("%s/%s/%s", crdName, resourceName, namespace)
}