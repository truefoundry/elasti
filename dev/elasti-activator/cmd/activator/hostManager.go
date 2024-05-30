package main

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Host struct {
	Namespace      string
	SourceService  string
	TargetService  string
	SourceHost     string
	TargetHost     string
	TrafficAllowed bool
}

type hostManager struct {
	logger                  *zap.Logger
	hosts                   sync.Map
	reEnableTrafficDuration time.Duration
}

var HostManager *hostManager

func InitHostManager(logger *zap.Logger) {
	HostManager = &hostManager{
		logger:                  logger.With(zap.String("component", "hostManager")),
		hosts:                   sync.Map{},
		reEnableTrafficDuration: 10 * time.Second,
	}
}

func (hm *hostManager) GetHost(req *http.Request) (*Host, error) {
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
		targetService = sourceService + "-pvt"
		sourceHost = hm.removeTrailingWildcardIfNeeded(sourceHost)
		sourceHost = hm.addHTTPIfNeeded(sourceHost)
		targetHost = hm.replaceServiceName(sourceHost, targetService)
		targetHost = hm.addHTTPIfNeeded(targetHost)
		trafficAllowed := true
		host = &Host{
			Namespace:      namespace,
			SourceService:  sourceService,
			TargetService:  targetService,
			SourceHost:     sourceHost,
			TargetHost:     targetHost,
			TrafficAllowed: trafficAllowed,
		}
		hm.hosts.Store(sourceService, host)
	}
	return host.(*Host), nil
}

func (hm *hostManager) DisableTrafficForHost(service string) {
	host, _ := hm.hosts.Load(service)
	if host.(*Host).TrafficAllowed {
		host.(*Host).TrafficAllowed = false
		hm.hosts.Store(service, host)
		hm.logger.Debug("Disabling traffic for host", zap.Any("service", service))
	}
	go hm.enableReEnableTrafficForHost(service)
}

func (hm *hostManager) EnableTrafficForHost(service string) {
	host, _ := hm.hosts.Load(service)
	if !host.(*Host).TrafficAllowed {
		host.(*Host).TrafficAllowed = true
		hm.hosts.Store(service, host)
		hm.logger.Debug("Enabling traffic for host", zap.Any("service", service))
	}
}

func (hm *hostManager) enableReEnableTrafficForHost(service string) {
	time.AfterFunc(hm.reEnableTrafficDuration, func() {
		hm.EnableTrafficForHost(service)
	})
}

func (hm *hostManager) extractNamespaceAndService(s string, internal bool) (string, string, error) {
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

func (hm *hostManager) addHTTPIfNeeded(serviceURL string) string {
	if !strings.HasPrefix(serviceURL, "http://") && !strings.HasPrefix(serviceURL, "https://") {
		return "http://" + serviceURL
	}
	return serviceURL
}

func (hm *hostManager) removeTrailingWildcardIfNeeded(serviceURL string) string {
	if strings.HasSuffix(serviceURL, "/*") {
		return strings.TrimSuffix(serviceURL, "/*")
	}
	return serviceURL
}

func (hm *hostManager) replaceServiceName(serviceURL, newServiceName string) string {
	parts := strings.Split(serviceURL, ".")
	if len(parts) < 3 {
		return serviceURL
	}
	parts[0] = newServiceName
	return strings.Join(parts, ".")
}
