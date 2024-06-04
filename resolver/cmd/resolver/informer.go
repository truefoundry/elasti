package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/truefoundry/elasti/pkg/messages"
	"go.uber.org/zap"

	"sync"
)

var creationLock = &sync.Mutex{}

type informerSVC struct {
	logger      *zap.Logger
	lockTimeout time.Duration
	locks       map[string]*sync.Mutex
}

var Informer *informerSVC

func InitInformer(logger *zap.Logger, lockTimeout time.Duration) {
	creationLock.Lock()
	locks := map[string]*sync.Mutex{}
	Informer = &informerSVC{
		logger:      logger.With(zap.String("component", "informer")),
		lockTimeout: lockTimeout,
		locks:       locks,
	}
}

// Inform send update to controller about the incoming requests
func (i *informerSVC) Inform(ns, svc string) {
	lockKey := fmt.Sprintf("%s.%s", svc, ns)
	if getLock := i.getLock(lockKey); !getLock {
		i.logger.Info("Controller already informed, nothing to do here.")
		return
	}
	i.logger.Debug("controller informer lock acquired", zap.String("key", lockKey))

	// Create the request body
	requestBody := messages.RequestCount{
		Count:     1,
		Svc:       svc,
		Namespace: ns,
	}

	// Marshal the request body to JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		i.logger.Error("Error marshalling request body", zap.Error(err))
		return
	}
	url := "http://elasti-operator-controller-service.elasti-operator-system.svc.cluster.local:8013/informer/incoming-request"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		i.logger.Error("Error creating request", zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		i.logger.Error("Error sending request", zap.Error(err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		i.logger.Error("Request failed with status code", zap.Int("status_code", resp.StatusCode))
		return
	}
	i.logger.Info("Request sent to controller", zap.Int("statusCode", resp.StatusCode), zap.Any("body", resp.Body))
}

// getLock check if the lock already taken, if yes, it returns false
// If not take, it takes a new lock and unlock it after the given duration
func (i *informerSVC) getLock(key string) bool {
	_, ok := i.locks[key]
	if ok {
		return false
	}
	i.locks[key] = &sync.Mutex{}
	i.locks[key].Lock()
	time.AfterFunc(i.lockTimeout, func() {
		i.locks[key].Unlock()
		i.logger.Debug("controller informer lock release", zap.String("key", key))
		delete(i.locks, key)
	})
	return true
}
