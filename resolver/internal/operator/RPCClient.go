package operator

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/truefoundry/elasti/pkg/messages"
	"go.uber.org/zap"

	"sync"
)

// OperatorClient is to communicate with the operator
type OperatorClient struct {
	logger *zap.Logger
	// retryDuration is the duration to wait before retrying the operator
	retryDuration time.Duration
	// rpcs is to keep track of the locks for different services
	rpcs sync.Map
	// operatorURL is the URL of the operator
	operatorURL string
	// incomingRequestEndpoint is the endpoint to send information about the incoming request
	incomingRequestEndpoint string
	// client is the http client
	client http.Client
}

// NewOperatorClient returns a new OperatorClient
func NewOperatorClient(logger *zap.Logger, retryDuration time.Duration) *OperatorClient {
	return &OperatorClient{
		logger:                  logger.With(zap.String("component", "operatorRPC")),
		retryDuration:           retryDuration,
		operatorURL:             "http://elasti-operator-controller-service.elasti-operator-system.svc.cluster.local:8013",
		incomingRequestEndpoint: "/informer/incoming-request",
		client:                  http.Client{},
	}
}

// SendIncomingRequestInfo send request details like service name to the operator
func (o *OperatorClient) SendIncomingRequestInfo(ns, svc string) {
	if _, ok := o.rpcs.Load(svc); ok {
		o.logger.Debug("Operator already informed about incoming requests", zap.String("service", svc))
		return
	}
	requestBody := messages.RequestCount{
		Count:     1,
		Svc:       svc,
		Namespace: ns,
	}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		o.logger.Error("Error marshalling request body for operatorRPC", zap.Error(err))
		return
	}
	url := o.operatorURL + o.incomingRequestEndpoint
	if req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData)); err != nil {
		o.logger.Error("Error creating request", zap.Error(err))
		return
	} else {
		req.Header.Set("Content-Type", "application/json")
		resp, err := o.client.Do(req)
		if err != nil {
			o.logger.Error("Error sending request", zap.Error(err))
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			o.logger.Error("Request failed with status code", zap.Int("status_code", resp.StatusCode))
			return
		}
		o.logger.Info("Request sent to controller", zap.Int("statusCode", resp.StatusCode), zap.Any("body", resp.Body))
		o.rpcs.Store(svc, true)
		go time.AfterFunc(o.retryDuration, o.getLockReleaseFunc(svc))
	}
}

// getLockReleaseFunc returns a function to release the lock, taken by the operatorRPC to rate limit the requests
func (o *OperatorClient) getLockReleaseFunc(svc string) func() {
	return func() {
		o.rpcs.Delete(svc)
	}
}
