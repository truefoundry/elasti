package operator

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"
	"truefoundry/elasti/resolver/internal/prom"

	"github.com/truefoundry/elasti/pkg/messages"
	"go.uber.org/zap"

	"sync"
)

// Client is to communicate with the operator
type Client struct {
	logger *zap.Logger
	// retryDuration is the duration to wait before retrying the operator
	retryDuration time.Duration
	// serviceRPCLocks is to keep track of the locks for different services
	serviceRPCLocks sync.Map
	// operatorURL is the URL of the operator
	operatorURL string
	// incomingRequestEndpoint is the endpoint to send information about the incoming request
	incomingRequestEndpoint string
	// client is the http client
	client http.Client
}

// NewOperatorClient returns a new OperatorClient
func NewOperatorClient(logger *zap.Logger, retryDuration time.Duration) *Client {
	return &Client{
		logger:                  logger.With(zap.String("component", "operatorRPC")),
		retryDuration:           retryDuration,
		operatorURL:             "http://elasti-operator-controller-service:8013",
		incomingRequestEndpoint: "/informer/incoming-request",
		client:                  http.Client{},
	}
}

// SendIncomingRequestInfo send request details like service name to the operator
func (o *Client) SendIncomingRequestInfo(ns, svc string) {
	lock, taken := o.getMutexForServiceRPC(svc)
	if taken {
		return
	}
	lock.Lock()
	defer time.AfterFunc(o.retryDuration, func() {
		o.releaseMutexForServiceRPC(svc)
	})

	requestBody := messages.RequestCount{
		Count:     1,
		Svc:       svc,
		Namespace: ns,
	}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		prom.OperatorRPCCounter.WithLabelValues(err.Error()).Inc()
		o.logger.Error("Error marshalling request body for operatorRPC", zap.Error(err))
		return
	}
	url := o.operatorURL + o.incomingRequestEndpoint
	if req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData)); err != nil {
		prom.OperatorRPCCounter.WithLabelValues(err.Error()).Inc()
		o.logger.Error("Error creating request", zap.Error(err))
		return
	} else {
		req.Header.Set("Content-Type", "application/json")
		resp, err := o.client.Do(req)
		if err != nil {
			prom.OperatorRPCCounter.WithLabelValues(err.Error()).Inc()
			o.logger.Error("Error sending request", zap.Error(err))
			return
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				prom.OperatorRPCCounter.WithLabelValues(err.Error()).Inc()
				o.logger.Error("Error closing body", zap.Error(err))
			}
		}(resp.Body)
		if resp.StatusCode != http.StatusOK {
			prom.OperatorRPCCounter.WithLabelValues(strconv.Itoa(resp.StatusCode)).Inc()
			o.logger.Error("Request failed with status code", zap.Int("status_code", resp.StatusCode))
			return
		}
		prom.OperatorRPCCounter.WithLabelValues("").Inc()
		o.logger.Info("Request sent to controller", zap.Int("statusCode", resp.StatusCode), zap.Any("body", resp.Body))
	}
}

func (o *Client) releaseMutexForServiceRPC(service string) {
	lock, loaded := o.serviceRPCLocks.Load(service)
	if !loaded {
		return
	}
	lock.(*sync.Mutex).Unlock()
	o.serviceRPCLocks.Delete(service)
}

func (o *Client) getMutexForServiceRPC(service string) (*sync.Mutex, bool) {
	m, loaded := o.serviceRPCLocks.LoadOrStore(service, &sync.Mutex{})
	return m.(*sync.Mutex), loaded
}
