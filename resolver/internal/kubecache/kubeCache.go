package kubecache

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	core_v1 "k8s.io/api/core/v1"
	networking_v1 "k8s.io/api/networking/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KubeCache struct {
	logger    *zap.Logger
	clientset kubernetes.Interface

	ingressCache sync.Map
	serviceCache sync.Map
}

func NewKubeCache(logger *zap.Logger, clientset kubernetes.Interface) *KubeCache {
	return &KubeCache{
		logger:    logger,
		clientset: clientset,

		ingressCache: sync.Map{},
		serviceCache: sync.Map{},
	}
}

func (kc *KubeCache) findServiceForIngressRequest(req *http.Request) (cache.ObjectName, networking_v1.ServiceBackendPort, bool) {
	var matchingService cache.ObjectName
	var matchinServicePort networking_v1.ServiceBackendPort
	matchLen := 0

	kc.ingressCache.Range(func(key any, value any) bool {
		ingressID := key.(cache.ObjectName)
		ingressSpec := value.(networking_v1.IngressSpec)

		for _, rule := range ingressSpec.Rules {
			if rule.Host != req.Host {
				continue
			}

			for _, path := range rule.HTTP.Paths {
				pathMatch := false

				switch *path.PathType {
				case networking_v1.PathTypePrefix:
					pathMatch = strings.HasPrefix(req.URL.Path, path.Path)

				case networking_v1.PathTypeExact:
					pathMatch = (req.URL.Path == path.Path)

				default:
					kc.logger.Info("Not supported PathType matches host", zap.String("PathType", string(*path.PathType)), zap.String("Host", req.Host))
				}

				if pathMatch {
					if (len(path.Path) > matchLen) || ((len(path.Path) == matchLen) && (*path.PathType == networking_v1.PathTypePrefix)) {
						matchingService = cache.ObjectName{
							Namespace: ingressID.Namespace,
							Name:      path.Backend.Service.Name,
						}

						matchinServicePort = path.Backend.Service.Port
						matchLen = len(path.Path)
					}
				}
			}
		}

		return true
	})

	found := len(matchingService.Name) != 0

	return matchingService, matchinServicePort, found
}

func (kc *KubeCache) GetServiceForRequest(req *http.Request) (string, string, int32, bool) {
	service, port, serviceFound := kc.findServiceForIngressRequest(req)

	if serviceFound {
		numericPort, portFound := kc.getNumericPortForService(service, port)

		if portFound {
			return service.Namespace, service.Name, numericPort, true
		}
	}

	return "", "", 0, false

}

func (kc *KubeCache) getNumericPortForService(serviceID cache.ObjectName, backendPort networking_v1.ServiceBackendPort) (int32, bool) {
	item, serviceFound := kc.serviceCache.Load(serviceID)

	if serviceFound {
		ports := item.([]core_v1.ServicePort)

		for _, port := range ports {
			if len(backendPort.Name) != 0 {
				if port.Name == backendPort.Name {
					return port.Port, true
				}
			} else {
				if port.Port == backendPort.Number {
					return port.Port, true
				}
			}
		}

	}

	return 0, false
}

func (kc *KubeCache) ServiceExists(namespace string, name string) bool {
	serviceID := cache.ObjectName{
		Namespace: namespace,
		Name:      name,
	}

	_, ok := kc.serviceCache.Load(serviceID)

	return ok
}

func (kc *KubeCache) Start(ctx context.Context) error {
	err := kc.watchIngresses(ctx)
	if err != nil {
		return err
	}

	err = kc.watchServices(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (kc *KubeCache) watchIngresses(ctx context.Context) error {
	ingressList, err := kc.clientset.NetworkingV1().Ingresses("").List(ctx, meta_v1.ListOptions{})
	if err != nil {
		return err
	}

	kc.logger.Debug("list: ingress", zap.Int("count", len(ingressList.Items)))

	for _, ingress := range ingressList.Items {
		key := cache.MetaObjectToName(&ingress)

		kc.ingressCache.Store(key, ingress.Spec)
		kc.logger.Debug("list: ingress", zap.String("name", ingress.Name), zap.String("namespace", ingress.Namespace))
	}

	ingressWatch, err := kc.clientset.NetworkingV1().Ingresses("").Watch(ctx, meta_v1.ListOptions{ResourceVersion: ingressList.ResourceVersion})
	if err != nil {
		return err
	}

	go func() {
		for event := range ingressWatch.ResultChan() {
			ingress := event.Object.(*networking_v1.Ingress)
			key := cache.MetaObjectToName(ingress)

			kc.logger.Debug("watch: ingress",
				zap.String("action", string(event.Type)),
				zap.String("name", ingress.Name),
				zap.String("namespace", ingress.Namespace),
			)

			switch event.Type {
			case watch.Added:
				fallthrough
			case watch.Modified:
				kc.ingressCache.Store(key, ingress.Spec)

			case watch.Deleted:
				kc.ingressCache.Delete(key)
			}
		}

		kc.logger.Debug("Stopping ingress watch")
	}()

	return nil
}

func (kc *KubeCache) watchServices(ctx context.Context) error {
	serviceList, err := kc.clientset.CoreV1().Services("").List(ctx, meta_v1.ListOptions{})
	if err != nil {
		return err
	}

	kc.logger.Debug("list: service", zap.Int("count", len(serviceList.Items)))

	for _, service := range serviceList.Items {
		key := cache.MetaObjectToName(&service)

		kc.serviceCache.Store(key, service.Spec.Ports)
		kc.logger.Debug("list: service", zap.String("name", service.Name), zap.String("namespace", service.Namespace))
	}

	serviceWatch, err := kc.clientset.CoreV1().Services("").Watch(ctx, meta_v1.ListOptions{ResourceVersion: serviceList.ResourceVersion})
	if err != nil {
		return err
	}

	go func() {
		for event := range serviceWatch.ResultChan() {
			service := event.Object.(*core_v1.Service)
			key := cache.MetaObjectToName(service)

			kc.logger.Debug("watch: service",
				zap.String("action", string(event.Type)),
				zap.String("name", service.Name),
				zap.String("namespace", service.Namespace),
			)

			switch event.Type {
			case watch.Added:
				fallthrough
			case watch.Modified:
				kc.serviceCache.Store(key, service.Spec.Ports)

			case watch.Deleted:
				kc.serviceCache.Delete(key)
			}
		}

		kc.logger.Debug("Stopping service watch")
	}()

	return nil
}
