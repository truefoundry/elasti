package hostManager

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

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
}

// NewHostManager returns a new HostManager
func NewHostManager(logger *zap.Logger, trafficReEnableDuration time.Duration) *HostManager {
	return &HostManager{
		logger:                  logger.With(zap.String("component", "hostManager")),
		hosts:                   sync.Map{},
		trafficReEnableDuration: trafficReEnableDuration,
	}
}

// GetHost returns the host details for incoming and outgoing requests
func (hm *HostManager) GetHost(req *http.Request) (*messages.Host, error) {
	var namespace, sourceService, targetService, sourceHost, targetHost string
	sourceHost = req.Host
	internal := true
	if values, ok := req.Header["X-Envoy-Decorator-Operation"]; ok {
		sourceHost = values[0]
		internal = false
	}
	namespace, sourceService, err := hm.extractNamespaceAndService(sourceHost, internal)
	if err != nil {
		return nil, err
	}
	host, ok := hm.hosts.Load(sourceService)
	if !ok {
		targetService = utils.GetPrivateSerivceName(sourceService)
		sourceHost = hm.removeTrailingWildcardIfNeeded(sourceHost)
		sourceHost = hm.addHTTPIfNeeded(sourceHost)
		targetHost = hm.replaceServiceName(sourceHost, targetService)
		targetHost = hm.addHTTPIfNeeded(targetHost)
		host = &messages.Host{
			Namespace:      namespace,
			SourceService:  sourceService,
			TargetService:  targetService,
			SourceHost:     sourceHost,
			TargetHost:     targetHost,
			TrafficAllowed: true,
		}
		hm.hosts.Store(sourceService, host)
	}
	return host.(*messages.Host), nil
}

// DisableTrafficForHost disables the traffic for the host
func (hm *HostManager) DisableTrafficForHost(hostName string) {
	if host, ok := hm.hosts.Load(hostName); ok && host.(*messages.Host).TrafficAllowed {
		host.(*messages.Host).TrafficAllowed = false
		hm.hosts.Store(hostName, host)
		hm.logger.Debug("Disabled traffic for host",
			zap.Any("hostName", hostName),
			zap.Any("trafficReEnableDuration", hm.trafficReEnableDuration))
		go time.AfterFunc(hm.trafficReEnableDuration, func() {
			hm.enableTrafficForHost(hostName)
		})
	}
}

// enableTrafficForHost enables the traffic for the host
func (hm *HostManager) enableTrafficForHost(hostName string) {
	if host, ok := hm.hosts.Load(hostName); ok && !host.(*messages.Host).TrafficAllowed {
		host.(*messages.Host).TrafficAllowed = true
		hm.hosts.Store(hostName, host)
		hm.logger.Debug("Enabled traffic for host", zap.Any("hostName", hostName))
	}
}

// extractNamespaceAndService extracts the namespace and service name from the host
func (hm *HostManager) extractNamespaceAndService(s string, internal bool) (string, string, error) {
	re := regexp.MustCompile(`(?P<service>[^.]+)\.(?P<namespace>[^.]+)\.svc\.cluster\.local:\d+/\*`)
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

// replaceServiceName replaces the service name in the service URL
func (hm *HostManager) replaceServiceName(serviceURL, newServiceName string) string {
	parts := strings.Split(serviceURL, ".")
	if len(parts) < 3 {
		return serviceURL
	}
	parts[0] = newServiceName
	return strings.Join(parts, ".")
}
