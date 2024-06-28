// Package informer helps you manage your informers/watches and gracefully start and stop them
// It checks health for them, and restrict only 1 informer is running for 1 resource for each crd
package informer

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"k8s.io/client-go/dynamic"

	ctrl "sigs.k8s.io/controller-runtime"
)

type (
	// Manager helps manage lifecycle of informer
	Manager struct {
		client              *kubernetes.Clientset
		dynamicClient       *dynamic.DynamicClient
		logger              *zap.Logger
		informers           sync.Map
		resyncPeriod        time.Duration
		healthCheckDuration time.Duration
		healthCheckStopChan chan struct{}
	}

	info struct {
		Informer cache.SharedInformer
		StopCh   chan struct{}
		Req      *RequestWatch
	}
	// RequestWatch is the request body sent to the informer
	RequestWatch struct {
		Req                  ctrl.Request
		ResourceName         string
		ResourceNamespace    string
		GroupVersionResource *schema.GroupVersionResource
		Handlers             cache.ResourceEventHandlerFuncs
	}
)

// NewInformerManager creates a new instance of the Informer Manager
func NewInformerManager(logger *zap.Logger, kConfig *rest.Config) *Manager {
	clientSet, err := kubernetes.NewForConfig(kConfig)
	if err != nil {
		logger.Fatal("Error connecting with kubernetes", zap.Error(err))
	}
	dynamicClient, err := dynamic.NewForConfig(kConfig)
	if err != nil {
		logger.Fatal("Error connecting with kubernetes", zap.Error(err))
	}

	return &Manager{
		client:        clientSet,
		dynamicClient: dynamicClient,
		logger:        logger.Named("InformerManager"),
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
		info, ok := value.(info)
		if ok {
			m.StopInformer(m.getKeyFromRequestWatch(info.Req))
		}
		return true
	})
	// Stop the health watch
	close(m.healthCheckStopChan)
	m.logger.Info("InformerManager stopped")
}

// StopForCRD is to close all the active informers for a perticular CRD
func (m *Manager) StopForCRD(crdName string) {
	m.logger.Info("Stopping Informer for CRD", zap.String("crd", crdName))
	// Loop through all the informers and stop them
	m.informers.Range(func(key, value interface{}) bool {
		// Check if key starts with the crdName
		if key.(string)[:len(crdName)] == crdName {
			info, ok := value.(info)
			if ok {
				m.StopInformer(m.getKeyFromRequestWatch(info.Req))
			}
		}
		return true
	})
	m.logger.Info("Informer stopped for CRD", zap.String("crd", crdName))
}

// StopInformer is to stop a informer for a resource
// It closes the shared informer for it and deletes it from the map
func (m *Manager) StopInformer(key string) {
	m.logger.Info("Stopping informer", zap.String("key", key))
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

func (m *Manager) monitorInformers() {
	m.informers.Range(func(key, value interface{}) bool {
		info, ok := value.(info)
		if ok {
			if !info.Informer.HasSynced() {
				m.logger.Info("Informer not synced", zap.String("key", key.(string)))
				m.StopInformer(m.getKeyFromRequestWatch(info.Req))
				m.enableInformer(info.Req)
			}
		}
		return true
	})
}

// AddDeploymentWatch is to add a watch on a deployment
func (m *Manager) AddDeploymentWatch(req ctrl.Request, deploymentName, namespace string, handlers cache.ResourceEventHandlerFuncs) {
	request := &RequestWatch{
		Req:               req,
		ResourceName:      deploymentName,
		ResourceNamespace: namespace,
		GroupVersionResource: &schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		},
		Handlers: handlers,
	}
	m.Add(request)
}

// Add is to add a watch on a resource
func (m *Manager) Add(req *RequestWatch) {
	m.logger.Info("Adding informer",
		zap.String("group", req.GroupVersionResource.Group),
		zap.String("version", req.GroupVersionResource.Version),
		zap.String("resource", req.GroupVersionResource.Resource),
		zap.String("resourceName", req.ResourceName),
		zap.String("resourceNamespace", req.ResourceNamespace),
		zap.String("crd", req.Req.String()),
	)
	m.enableInformer(req)
}

// enableInformer is to enable the informer for a resource
func (m *Manager) enableInformer(req *RequestWatch) {
	defer func() {
		if rErr := recover(); rErr != nil {
			m.logger.Error("Recovered from panic", zap.Any("recovered", rErr))
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			m.logger.Error("Panic stack trace", zap.ByteString("stacktrace", buf[:n]))
		}
	}()
	key := m.getKeyFromRequestWatch(req)
	// Proceed only if the informer is not already running, we verify by checking the map
	if _, ok := m.informers.Load(key); ok {
		m.logger.Info("Informer already running", zap.String("key", key))
		return
	}

	ctx := context.Background()
	// Create an informer for the resource
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (kRuntime.Object, error) {
				return m.dynamicClient.Resource(*req.GroupVersionResource).Namespace(req.ResourceNamespace).List(ctx, metav1.ListOptions{
					FieldSelector: "metadata.name=" + req.ResourceName,
				})
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return m.dynamicClient.Resource(*req.GroupVersionResource).Namespace(req.ResourceNamespace).Watch(ctx, metav1.ListOptions{
					FieldSelector: "metadata.name=" + req.ResourceName,
				})
			},
		},
		&unstructured.Unstructured{},
		0,
	)
	// We pass the handlers we received as a parameter
	_, err := informer.AddEventHandler(req.Handlers)
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
		Informer: informer,
		StopCh:   informerStop,
		Req:      req,
	})

	// Wait for the cache to syncß
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		m.logger.Error("Failed to sync informer", zap.String("key", key))
		return
	}
	m.logger.Info("Informer started", zap.String("key", key))
}

// getKeyFromRequestWatch is to get the key for the informer map using namespace and resource name from the request
// CRDname.resourcerName.Namespace
func (m *Manager) getKeyFromRequestWatch(req *RequestWatch) string {
	return fmt.Sprintf("%s/%s/%s/%s",
		strings.ToLower(req.ResourceNamespace),
		strings.ToLower(req.Req.Name),
		strings.ToLower(req.GroupVersionResource.Resource),
		strings.ToLower(req.ResourceName))
}
