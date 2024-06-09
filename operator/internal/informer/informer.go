// Package informer helps you manage your informers/watches and gracefully start and stop them
// It checks health for them, and restrict only 1 informer is running for 1 resource for each crd
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

// Manager helps manage lifecycle of informer
type Manager struct {
	client              *kubernetes.Clientset
	logger              *zap.Logger
	informers           sync.Map
	resyncPeriod        time.Duration
	healthCheckDuration time.Duration
	healthCheckStopChan chan struct{}
}

type info struct {
	Informer cache.SharedIndexInformer
	StopCh   chan struct{}
	Handlers cache.ResourceEventHandlerFuncs
}

// NewInformerManager creates a new instance of the Informer Manager
func NewInformerManager(logger *zap.Logger, kConfig *rest.Config) *Manager {
	clientSet, err := kubernetes.NewForConfig(kConfig)
	if err != nil {
		logger.Fatal("Error connecting with kubernetes", zap.Error(err))
	}
	return &Manager{
		client: clientSet,
		logger: logger.Named("InformerManager"),
		// ResyncPeriod is the proactive resync we do, even when no events are received by the informer.
		resyncPeriod:        0,
		healthCheckDuration: 5 * time.Second,
		healthCheckStopChan: make(chan struct{}),
	}
}

// Start is to initiate a health check on all the running informers
// It uses HasSynced if a informer is not synced, if not, it restarts it
func (m *Manager) Start() {
	m.logger.Info("Starting InformerManager")
	go wait.Until(m.monitorInformers, m.healthCheckDuration, m.healthCheckStopChan)
}

// Stop is to close all the active informers and close the health monitor
func (m *Manager) Stop() {
	m.logger.Info("Stopping InformerManager")
	// Loop through all the informers and stop them
	m.informers.Range(func(key, value interface{}) bool {
		_, ok := value.(info)
		if ok {
			m.StopInformer(parseInformerKey(key.(string)))
		}
		return true
	})
	// Stop the health watch
	close(m.healthCheckStopChan)
	m.logger.Info("InformerManager stopped")
}

func (m *Manager) monitorInformers() {
	m.informers.Range(func(key, value interface{}) bool {
		informerInfo, ok := value.(info)
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
	key := getInformerKey(crdName, deploymentName, namespace)
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
	m.informers.Store(key, info{
		StopCh:   informerStop,
		Handlers: handlers,
		Informer: informer,
	})
	m.logger.Info("Informer started", zap.String("key", key))
}

// StopInformer is to stop a informer for a resource
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
	informerInfo, ok := value.(info)
	if !ok {
		m.logger.Error("Failed to cast WatchInfo", zap.String("key", key))
		return
	}

	// Close the informer, delete it from the map
	close(informerInfo.StopCh)
	m.informers.Delete(key)
	m.logger.Info("Informer stopped", zap.String("key", key))
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
