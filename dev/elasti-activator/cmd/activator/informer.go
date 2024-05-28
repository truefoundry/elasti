package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"sync"
)

type informerSVC struct {
	logger      *zap.Logger
	lockTimeout time.Duration
	locks       map[string]*sync.Mutex
}

var Informer *informerSVC

func InitInformer(logger *zap.Logger, lockTimeout time.Duration) {
	locks := map[string]*sync.Mutex{}
	Informer = &informerSVC{
		logger:      logger,
		lockTimeout: lockTimeout,
		locks:       locks,
	}
}

type RequestCount struct {
	Count     int    `json:"count"`
	Svc       string `json:"svc"`
	Namespace string `json:"namespace"`
}

// Inform send update to controller about the incoming requests
func (i *informerSVC) Inform(ns, svc string) {
	lockKey := fmt.Sprintf("%s.%s", svc, ns)
	if getLock := i.getLock(lockKey); !getLock {
		i.logger.Info("Controller already informed, nothing to do here.")
		return
	}
	i.logger.Debug("Lock Acquired", zap.String("key", lockKey))

	// Create the request body
	requestBody := RequestCount{
		Count:     1,
		Svc:       svc,
		Namespace: ns,
	}

	// Marshal the request body to JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		fmt.Printf("Error marshalling request body: %v\n", err)
		return
	}
	url := "http://elasti-operator-controller-manager-metrics-service.elasti-operator-system.svc.cluster.local:8080/request-count"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending request: %v\n", err)
		return
	}
	defer resp.Body.Close()
	fmt.Printf("Response from service: %s\n", resp.Body)
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
		i.logger.Debug("Lock Released", zap.String("key", key))
		delete(i.locks, key)
	})
	return true
}
