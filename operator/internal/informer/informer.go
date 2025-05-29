// Package informer helps you manage your informers/watches and gracefully start and stop them
// It checks health for them, and restrict only 1 informer is running for 1 resource for each crd
package informer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"truefoundry/elasti/operator/internal/prom"

	"github.com/truefoundry/elasti/pkg/values"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"k8s.io/client-go/dynamic"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// TODO: Move to configMap
	resolverNamespace      = "elasti"
	resolverDeploymentName = "elasti-resolver"
	resolverServiceName    = "elasti-resolver-service"
	resolverPort           = 8012
)

type (
	// Manager helps manage lifecycle of informer
	Manager struct {
		client              *kubernetes.Clientset
		dynamicClient       *dynamic.DynamicClient
		logger              *zap.Logger
		informers           sync.Map
		resolver            info
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
		ElastiServiceNamespacedName types.NamespacedName
		ResourceName                string
		ResourceNamespace           string
		GroupVersionResource        *schema.GroupVersionResource
		Handlers                    cache.ResourceEventHandlerFuncs
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
		resyncPeriod:        5 * time.Minute,
		healthCheckDuration: 5 * time.Second,
		healthCheckStopChan: make(chan struct{}),
	}
}

func (m *Manager) InitializeResolverInformer(handlers cache.ResourceEventHandlerFuncs) error {
	deploymentGVR := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}

	m.resolver.Informer = cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc: func(_ metav1.ListOptions) (kRuntime.Object, error) {
				return m.dynamicClient.Resource(deploymentGVR).Namespace(resolverNamespace).List(context.Background(), metav1.ListOptions{
					FieldSelector: "metadata.name=" + resolverDeploymentName,
				})
			},
			WatchFunc: func(_ metav1.ListOptions) (watch.Interface, error) {
				return m.dynamicClient.Resource(deploymentGVR).Namespace(resolverNamespace).Watch(context.Background(), metav1.ListOptions{
					FieldSelector: "metadata.name=" + resolverDeploymentName,
				})
			},
		},
		&unstructured.Unstructured{},
		m.resyncPeriod,
	)

	_, err := m.resolver.Informer.AddEventHandler(handlers)
	if err != nil {
		m.logger.Error("Failed to add event handler", zap.Error(err))
		return fmt.Errorf("failed to add event handler: %w", err)
	}

	m.resolver.StopCh = make(chan struct{})
	go m.resolver.Informer.Run(m.resolver.StopCh)

	if !cache.WaitForCacheSync(m.resolver.StopCh, m.resolver.Informer.HasSynced) {
		m.logger.Error("Failed to sync informer", zap.String("key", m.getKeyFromRequestWatch(m.resolver.Req)))
		return errors.New("failed to sync resolver informer")
	}
	m.logger.Info("Resolver informer started")
	return nil
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
	m.informers.Range(func(_, value interface{}) bool {
		info, ok := value.(info)
		if ok {
			err := m.StopInformer(m.getKeyFromRequestWatch(info.Req))
			if err != nil {
				m.logger.Error("failed to stop informer", zap.Error(err))
			}
		}
		return true
	})
	// Stop the health watch
	close(m.healthCheckStopChan)
	m.logger.Info("InformerManager stopped")
}

// StopForCRD is to close all the active informers for a perticular CRD
func (m *Manager) StopForCRD(crdName string) {
	// Loop through all the informers and stop them
	var wg sync.WaitGroup
	m.informers.Range(func(key, value interface{}) bool {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Check if key starts with the crdName
			if key.(string)[:len(crdName)] == crdName {
				info, ok := value.(info)
				if ok {
					if err := m.StopInformer(m.getKeyFromRequestWatch(info.Req)); err != nil {
						m.logger.Error("Failed to stop informer", zap.Error(err))
					}
					m.logger.Info("Stopped informer", zap.String("key", key.(string)))
				}
			}
		}()
		return true
	})
	wg.Wait()
}

// StopInformer is to stop a informer for a resource
// It closes the shared informer for it and deletes it from the map
func (m *Manager) StopInformer(key string) (err error) {
	defer func() {
		errStr := values.Success
		if err != nil {
			errStr = err.Error()
		}
		prom.InformerCounter.WithLabelValues(key, "stop", errStr).Inc()
	}()
	value, ok := m.informers.Load(key)
	if !ok {
		return fmt.Errorf("informer not found, already stopped for key: %s", key)
	}

	// We need to verify if the informer exists in the map
	informerInfo, ok := value.(info)
	if !ok {
		return fmt.Errorf("failed to cast WatchInfo for key: %s", key)
	}

	// Close the informer, delete it from the map
	close(informerInfo.StopCh)
	m.informers.Delete(key)
	prom.InformerGauge.WithLabelValues(key).Dec()
	return nil
}

func (m *Manager) monitorInformers() {
	m.informers.Range(func(key, value interface{}) bool {
		info, ok := value.(info)
		if ok {
			if !info.Informer.HasSynced() {
				m.logger.Info("Informer not synced", zap.String("key", key.(string)))
				err := m.StopInformer(m.getKeyFromRequestWatch(info.Req))
				if err != nil {
					m.logger.Error("Error in stopping informer", zap.Error(err))
				}
				err = m.enableInformer(info.Req)
				if err != nil {
					m.logger.Error("Error in enabling informer", zap.Error(err))
				}
			}
		}
		return true
	})
}

// WatchDeployment is to add a watch on a deployment
func (m *Manager) WatchDeployment(req ctrl.Request, deploymentName, namespace string, handlers cache.ResourceEventHandlerFuncs) error {
	request := &RequestWatch{
		ElastiServiceNamespacedName: req.NamespacedName,
		ResourceName:                deploymentName,
		ResourceNamespace:           namespace,
		GroupVersionResource: &schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		},
		Handlers: handlers,
	}
	return m.Add(request)
}

// Add is to add a watch on a resource
func (m *Manager) Add(req *RequestWatch) (err error) {
	key := m.getKeyFromRequestWatch(req)
	defer func() {
		errStr := values.Success
		if err != nil {
			errStr = err.Error()
		}
		prom.InformerCounter.WithLabelValues(key, "add", errStr).Inc()
	}()
	m.logger.Info("Adding informer",
		zap.String("group", req.GroupVersionResource.Group),
		zap.String("version", req.GroupVersionResource.Version),
		zap.String("resource", req.GroupVersionResource.Resource),
		zap.String("resourceName", req.ResourceName),
		zap.String("resourceNamespace", req.ResourceNamespace),
		zap.String("crd", req.ElastiServiceNamespacedName.String()),
	)

	// Proceed only if the informer is not already running, we verify by checking the map
	if _, ok := m.informers.Load(key); ok {
		m.logger.Info("Informer already running", zap.String("key", key))
		return nil
	}

	//TODO: Check if the resource exists
	if err = m.verifyTargetExist(req); err != nil {
		return fmt.Errorf("target not found: %w", err)
	}

	if err = m.enableInformer(req); err != nil {
		return fmt.Errorf("failed to enable to informer: %w", err)
	}
	prom.InformerGauge.WithLabelValues(key).Inc()
	return nil
}

// enableInformer is to enable the informer for a resource
func (m *Manager) enableInformer(req *RequestWatch) error {
	ctx := context.Background()
	// Create an informer for the resource
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc: func(_ metav1.ListOptions) (kRuntime.Object, error) {
				return m.dynamicClient.Resource(*req.GroupVersionResource).Namespace(req.ResourceNamespace).List(ctx, metav1.ListOptions{
					FieldSelector: "metadata.name=" + req.ResourceName,
				})
			},
			WatchFunc: func(_ metav1.ListOptions) (watch.Interface, error) {
				return m.dynamicClient.Resource(*req.GroupVersionResource).Namespace(req.ResourceNamespace).Watch(ctx, metav1.ListOptions{
					FieldSelector: "metadata.name=" + req.ResourceName,
				})
			},
		},
		&unstructured.Unstructured{},
		m.resyncPeriod,
	)
	// We pass the handlers we received as a parameter
	_, err := informer.AddEventHandler(req.Handlers)
	if err != nil {
		m.logger.Error("Error creating informer handler", zap.Error(err))
		return fmt.Errorf("enableInformer: %w", err)
	}
	// This channel is used to stop the informer
	// We add it in the informers map, so we can stop it when required
	informerStop := make(chan struct{})
	go informer.Run(informerStop)
	// Store the informer in the map
	// This is used to manage the lifecycle of the informer
	// Recover it in case it's not syncing, this is why we also store the handlers
	// Stop it when the CRD or the operator is deleted
	key := m.getKeyFromRequestWatch(req)
	m.informers.Store(key, info{
		Informer: informer,
		StopCh:   informerStop,
		Req:      req,
	})

	// Wait for the cache to syncß
	if !cache.WaitForCacheSync(informerStop, informer.HasSynced) {
		m.logger.Error("Failed to sync informer", zap.String("key", key))
		return errors.New("failed to sync informer")
	}
	m.logger.Info("Informer started", zap.String("key", key))
	return nil
}

// getKeyFromRequestWatch is to get the key for the informer map using namespace and resource name from the request
// CRDname.resourcerName.Namespace
func (m *Manager) getKeyFromRequestWatch(req *RequestWatch) string {
	return fmt.Sprintf("%s/%s/%s/%s",
		strings.ToLower(req.ElastiServiceNamespacedName.Name),      // CRD Name
		strings.ToLower(req.ElastiServiceNamespacedName.Namespace), // Namespace
		strings.ToLower(req.GroupVersionResource.Resource),         // Resource Type
		strings.ToLower(req.ResourceName))                          // Resource Name
}

type KeyParams struct {
	Namespace    string
	CRDName      string
	Resource     string
	ResourceName string
}

// GetKey is to get the key for the informer map using namespace and resource name
func (m *Manager) GetKey(param KeyParams) string {
	return fmt.Sprintf("%s/%s/%s/%s",
		strings.ToLower(param.CRDName),      // CRD Name
		strings.ToLower(param.Namespace),    // Namespace
		strings.ToLower(param.Resource),     // Resource Type
		strings.ToLower(param.ResourceName)) // Resource Name
}

// verifyTargetExist is to verify if the target resource exists
func (m *Manager) verifyTargetExist(req *RequestWatch) error {
	if _, err := m.dynamicClient.Resource(*req.GroupVersionResource).Namespace(req.ResourceNamespace).Get(context.Background(), req.ResourceName, metav1.GetOptions{}); err != nil {
		return fmt.Errorf("resource doesn't exist: %w | resource name: %v | resource type: %v | resource namespace: %v", err, req.ResourceName, req.GroupVersionResource.Resource, req.ResourceNamespace)
	}
	return nil
}
