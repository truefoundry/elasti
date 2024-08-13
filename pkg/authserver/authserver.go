package authserver

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/imroc/req/v3"
)

type authServerImpl struct {
	client *req.Client
}
type AuthServer interface {
	GetSentryAuthData(authServerURL, tenantName string) (*SentryAuthInfo, error)
}

var (
	lock               = &sync.Mutex{}
	authServerInstance AuthServer
)

var GetAuthServerClient = func() AuthServer {
	lock.Lock()
	defer lock.Unlock()
	if authServerInstance == nil {
		client := req.C()
		client.SetCommonContentType("application/json")
		return authServerImpl{
			client: client,
		}
	}

	return authServerInstance
}

func (as authServerImpl) GetSentryAuthData(authServerURL, tenantName string) (*SentryAuthInfo, error) {
	queryParams := map[string]string{"serviceName": "elasti"}
	if tenantName != "" {
		queryParams["tenantName"] = tenantName
	}
	sentryAuth := SentryAuthInfo{}
	resp, err := as.client.R().
		SetQueryParams(queryParams).
		Get(fmt.Sprintf("%s/api/v1/tenants/sentry-auth-data", authServerURL))

	if err != nil {
		return nil, fmt.Errorf("GetSentryAuthData: %w", err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("GetSentryAuthData: %w", err)
	}
	if !resp.IsSuccessState() {
		return nil, fmt.Errorf("GetSentryAuthData: %s, Status Code: %d", body, resp.StatusCode)
	}

	if err := json.Unmarshal(body, &sentryAuth); err != nil {
		return nil, fmt.Errorf("GetSentryAuthData: %w", err)
	}
	return &sentryAuth, nil
}
