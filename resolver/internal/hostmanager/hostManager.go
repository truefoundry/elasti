package hostmanager

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/truefoundry/elasti/resolver/internal/kubecache"
	"github.com/truefoundry/elasti/resolver/internal/prom"

	"github.com/truefoundry/elasti/pkg/logger"
	"github.com/truefoundry/elasti/pkg/utils"

	"github.com/truefoundry/elasti/pkg/messages"
	"go.uber.org/zap"
)

// HostManager is to manage the hosts, and their traffic
// It is used to process incoming requests and cache the host details in "hosts" map
// For further requests, the cache is used to get the host details
type HostManager struct {
	logger                  *zap.Logger
	hosts                   sync.Map
	trafficReEnableDuration time.Duration
	headerForHost           string
	kubeCache               *kubecache.KubeCache
}

var ErrServiceNotFound = errors.New("service not found")

// NewHostManager returns a new HostManager
func NewHostManager(logger *zap.Logger, trafficReEnableDuration time.Duration, headerForHost string, kubeCache *kubecache.KubeCache) *HostManager {
	return &HostManager{
		logger:                  logger.With(zap.String("component", "hostManager")),
		hosts:                   sync.Map{},
		trafficReEnableDuration: trafficReEnableDuration,
		headerForHost:           headerForHost,
		kubeCache:               kubeCache,
	}
}

// GetHost returns the host details for incoming and outgoing requests
func (hm *HostManager) GetHost(req *http.Request) (*messages.Host, error) {
	incomingHost := req.Host
	if values, ok := req.Header[hm.headerForHost]; ok {
		incomingHost = values[0]
	}
	host, ok := hm.hosts.Load(incomingHost)
	if !ok {
		namespace, sourceService, servicePort, ingressFound := hm.kubeCache.GetServiceForRequest(req)

		if !ingressFound {
			var err error

			sourceService, namespace, err = hm.extractNamespaceAndService(incomingHost)
			if err != nil {
				prom.HostExtractionCounter.WithLabelValues("error", incomingHost, hm.headerForHost, err.Error()).Inc()
				return &messages.Host{}, err
			}

			if !hm.kubeCache.ServiceExists(namespace, sourceService) {
				err = fmt.Errorf("%w: %s/%s", ErrServiceNotFound, namespace, sourceService)

				prom.HostExtractionCounter.WithLabelValues("error", incomingHost, hm.headerForHost, err.Error()).Inc()
				return &messages.Host{}, err
			}
		}

		targetService := utils.GetPrivateServiceName(sourceService)
		sourceHost := hm.removeTrailingWildcardIfNeeded(incomingHost)
		sourceHost = hm.removeTrailingPathIfNeeded(sourceHost)
		sourceHost = hm.addHTTPIfNeeded(sourceHost)

		var targetHost string
		if ingressFound {
			targetHost = hm.createServiceURI(targetService, namespace, servicePort)
		} else {
			targetHost = hm.replaceServiceName(sourceHost, targetService)
		}

		targetHost = hm.addHTTPIfNeeded(targetHost)
		newHost := &messages.Host{
			IncomingHost:   incomingHost,
			Namespace:      namespace,
			SourceService:  sourceService,
			TargetService:  targetService,
			SourceHost:     sourceHost,
			TargetHost:     targetHost,
			TrafficAllowed: true,
		}
		hm.hosts.Store(incomingHost, newHost)
		prom.HostExtractionCounter.WithLabelValues("cache-miss", incomingHost, hm.headerForHost, "").Inc()
		return newHost, nil
	}
	prom.HostExtractionCounter.WithLabelValues("cache-hit", incomingHost, hm.headerForHost, "").Inc()
	return host.(*messages.Host), nil
}

// DisableTrafficForHost disables the traffic for the host
func (hm *HostManager) DisableTrafficForHost(hostName string) {
	if host, ok := hm.hosts.Load(hostName); ok && host.(*messages.Host).TrafficAllowed {
		host.(*messages.Host).TrafficAllowed = false
		hm.hosts.Store(hostName, host)
		hm.logger.Debug("Disabled traffic for host",
			zap.String("hostName", logger.MaskMiddle(hostName, 4, 4)),
			zap.Duration("trafficReEnableDuration", hm.trafficReEnableDuration))
		go time.AfterFunc(hm.trafficReEnableDuration, func() {
			hm.enableTrafficForHost(hostName)
		})
		prom.TrafficSwitchCounter.WithLabelValues(hostName, "disabled").Inc()
	}
}

// enableTrafficForHost enables the traffic for the host
func (hm *HostManager) enableTrafficForHost(hostName string) {
	if host, ok := hm.hosts.Load(hostName); ok && !host.(*messages.Host).TrafficAllowed {
		host.(*messages.Host).TrafficAllowed = true
		hm.hosts.Store(hostName, host)
		hm.logger.Debug("Enabled traffic for host", zap.Any("hostName", logger.MaskMiddle(hostName, 4, 4)))
		prom.TrafficSwitchCounter.WithLabelValues(hostName, "enabled").Inc()
	}
}

func (hm *HostManager) extractNamespaceAndService(url string) (string, string, error) {
	// Define regular expression patterns for different Kubernetes internal URL formats
	patterns := []string{
		`http://([a-zA-Z0-9-]+)\.([a-zA-Z0-9-]+)\.svc\.cluster\.local:\d+/\*`,
		`([a-zA-Z0-9-]+)\.([a-zA-Z0-9-]+)\.svc\.cluster\.local:\d+/\*`,
		`http://([a-zA-Z0-9-]+)\.([a-zA-Z0-9-]+)\.svc\.cluster\.local:\d+`,
		`([a-zA-Z0-9-]+)\.([a-zA-Z0-9-]+)\.svc\.cluster\.local:\d+`,
		`http://([a-zA-Z0-9-]+)\.([a-zA-Z0-9-]+)\.svc\.cluster\.local`,
		`([a-zA-Z0-9-]+)\.([a-zA-Z0-9-]+)\.svc\.cluster\.local`,
		`http://([a-zA-Z0-9-]+)\.([a-zA-Z0-9-]+)\.svc`,
		`([a-zA-Z0-9-]+)\.([a-zA-Z0-9-]+)\.svc`,
		`http://([a-zA-Z0-9-]+)\.([a-zA-Z0-9-]+)`,
		`([a-zA-Z0-9-]+)\.([a-zA-Z0-9-]+)`,
		`http://([a-zA-Z0-9-]+)\.svc\.cluster\.local`,
		`([a-zA-Z0-9-]+)\.svc\.cluster\.local`,
		`http://([a-zA-Z0-9-]+)\.svc`,
		`([a-zA-Z0-9-]+)\.svc`,
		`http://([a-zA-Z0-9-]+)`,
		`([a-zA-Z0-9-]+)`,
	}
	var serviceName, namespace string
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(url)
		if len(matches) == 3 {
			serviceName = matches[1]
			namespace = matches[2]
			return serviceName, namespace, nil
		} else if len(matches) == 2 {
			serviceName = matches[1]
			namespace = "default"
			return serviceName, namespace, fmt.Errorf("namespace not found in URL: %s", logger.MaskMiddle(url, 4, 4))
		}
	}
	return "", "", fmt.Errorf("invalid Kubernetes URL: %s", logger.MaskMiddle(url, 4, 4))
}

// addHTTPIfNeeded adds http if not present in the service URL
func (hm *HostManager) addHTTPIfNeeded(serviceURL string) string {
	if !strings.HasPrefix(serviceURL, "http://") && !strings.HasPrefix(serviceURL, "https://") {
		return "http://" + serviceURL
	}
	return serviceURL
}

// removeTrailingWildcardIfNeeded removes the trailing wildcard if present in the service URL
func (hm *HostManager) removeTrailingWildcardIfNeeded(serviceURL string) string {
	if strings.HasSuffix(serviceURL, "/*") {
		return strings.TrimSuffix(serviceURL, "/*")
	}
	return serviceURL
}

func (hm *HostManager) removeTrailingPathIfNeeded(serviceURL string) string {
	if idx := strings.Index(serviceURL, "/"); idx != -1 {
		return serviceURL[:idx]
	}
	return serviceURL
}

// replaceServiceName replaces the service name in the service URL
func (hm *HostManager) replaceServiceName(serviceURL, newServiceName string) string {
	parts := strings.Split(serviceURL, ".")
	if len(parts) < 3 {
		return serviceURL
	}
	parts[0] = newServiceName
	return strings.Join(parts, ".")
}

func (hm *HostManager) createServiceURI(name string, namespace string, port int32) string {
	uri := name + "." + namespace + ".svc.cluster.local:" + strconv.Itoa(int(port))

	return uri
}
