package controller

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"k8s.io/apimachinery/pkg/util/wait"
)

type InformerManager struct {
	client       *kubernetes.Clientset
	logger       *zap.Logger
	informers    sync.Map
	resyncPeriod time.Duration
}

type InformerInfo struct {
	Informer cache.SharedIndexInformer
	StopCh   chan struct{}
	Handlers cache.ResourceEventHandlerFuncs
	Active   bool
}

func NewInformerManager(logger *zap.Logger, kConfig *rest.Config) *InformerManager {
	clientset, cerr := kubernetes.NewForConfig(kConfig)
	if cerr != nil {
		logger.Fatal("Error connecting with kubernetes", zap.Error(cerr))
	}
	return &InformerManager{
		client:       clientset,
		logger:       logger.With(zap.String("component", "watcher")),
		resyncPeriod: 0,
	}
}

func (m *InformerManager) Start() {
	m.logger.Info("Starting InformerManager")
	go wait.Until(m.monitorInformers, 2*time.Second, make(<-chan struct{}))
}

func (m *InformerManager) monitorInformers() {
	m.informers.Range(func(key, value interface{}) bool {
		informerInfo, ok := value.(InformerInfo)
		if ok {
			if !informerInfo.Informer.HasSynced() {
				m.logger.Info("Informer not synced", zap.String("deployment_name", key.(string)))
				namespace, name := parseKey(key.(string))
				m.StopInformer(name, namespace)
				m.enableInformer(name, namespace, informerInfo.Handlers)
			}
		}
		return true
	})
}

func (m *InformerManager) Add(deploymentName, namespace string, handlers cache.ResourceEventHandlerFuncs) {
	m.logger.Info("Adding Deployment informer", zap.String("deployment_name", deploymentName))
	go m.enableInformer(deploymentName, namespace, handlers)
}

func (m *InformerManager) enableInformer(deploymentName, namespace string, handlers cache.ResourceEventHandlerFuncs) {
	key := getKey(namespace, deploymentName)
	defer func() {
		if r := recover(); r != nil {
			m.logger.Error("Recovered from panic", zap.Any("error", r))
			m.StopInformer(deploymentName, namespace)
			m.enableInformer(deploymentName, namespace, handlers)
		}
	}()
	if i, ok := m.informers.Load(key); ok && i.(InformerInfo).Active {
		m.logger.Info("Informer already running", zap.String("key", key))
		return
	}
	factory := informers.NewSharedInformerFactory(m.client, m.resyncPeriod)
	informer := factory.Apps().V1().Deployments().Informer()
	informer.AddEventHandler(handlers)
	informerStop := make(chan struct{})
	go informer.Run(informerStop)
	m.informers.Store(key, InformerInfo{
		Active:   true,
		StopCh:   informerStop,
		Handlers: handlers,
		Informer: informer,
	})
	m.logger.Info("Informer started", zap.String("key", key))
}

func (m *InformerManager) StopInformer(deploymentName, namespace string) {
	key := getKey(namespace, deploymentName)
	m.logger.Info("Stopping Deployment watch", zap.String("key", key))
	value, ok := m.informers.Load(key)
	if !ok {
		m.logger.Info("Informer not found", zap.String("key", key))
		return
	}

	informerInfo, ok := value.(InformerInfo)
	if !ok {
		m.logger.Error("Failed to cast WatchInfo", zap.String("key", key))
		return
	}

	close(informerInfo.StopCh)
	m.informers.Delete(key)
}

func parseKey(key string) (namespace, name string) {
	parts := strings.Split(key, "/")
	return parts[0], parts[1]
}

func getKey(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}
