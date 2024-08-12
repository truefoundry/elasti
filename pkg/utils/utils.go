package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/truefoundry/elasti/pkg/authserver"
	"go.uber.org/zap"
)

const (
	prefix                = "elasti-"
	privateServicePostfix = "-pvt"
	endpointSlicePostfix  = "-endpointslice-to-resolver"
)

var errInvalidAPIVersion = errors.New("invalid API version")

// GetPrivateSerivceName returns a private service name for a given public service name
// This generates a hash of the public service name and appends it to the private service name
// This way it decrease the chances of user having a same name, however, to be noted, the has will always be the same
// if the public service name is same
func GetPrivateSerivceName(publicSVCName string) string {
	hash := sha256.New()
	hash.Write([]byte(publicSVCName))
	hashed := hex.EncodeToString(hash.Sum(nil))
	pvtName := prefix + publicSVCName + privateServicePostfix + "-" + string(hashed)[:10]
	return pvtName
}

// GetEndpointSliceToResolverName returns an endpoint slice name for a given service name
func GetEndpointSliceToResolverName(serviceName string) string {
	hash := sha256.New()
	hash.Write([]byte(serviceName))
	hashed := hex.EncodeToString(hash.Sum(nil))
	return prefix + serviceName + endpointSlicePostfix + "-" + string(hashed)[:10]
}

// ParseAPIVersion returns the group, version
func ParseAPIVersion(apiVersion string) (group, version string, err error) {
	if apiVersion == "" {
		return "", "", errInvalidAPIVersion
	}
	split := strings.Split(apiVersion, "/")
	if len(split) != 2 {
		return "", "", errInvalidAPIVersion
	}
	return split[0], split[1], nil
}

func GetSentryAuthData(authServerURL, tenantName string) (*authserver.SentryAuthInfo, error) {
	if authServerURL == "" {
		return nil, fmt.Errorf("GetSentryAuthData: authServerURL is empty")
	}
	authServerClient := authserver.GetAuthServerClient()
	authData, err := authServerClient.GetSentryAuthData(authServerURL, tenantName)
	if err != nil {
		return nil, fmt.Errorf("GetSentryAuthData: %w", err)
	}
	return authData, nil
}


func InitializeSentry(logger *zap.Logger, authData *authserver.SentryAuthInfo, component string, tenantName string) {
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:         authData.Dsn,
		Environment: authData.Environment,
	}); err != nil {
		logger.Error("initializeSentry: Sentry initialization failed", zap.Error(err))
		return
	}
	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("tenant", tenantName)
		scope.SetTag("component", component)
	})
	logger.Info("initializeSentry: Sentry initialized")
}