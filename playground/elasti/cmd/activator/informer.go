package main

import (
	"fmt"
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

// Inform send update to controller about the incoming requests
func (i *informerSVC) Inform(ns, svc string) {
	lockKey := fmt.Sprintf("%s.%s", svc, ns)
	if getLock := i.getLock(lockKey); !getLock {
		i.logger.Info("Controller already informed, nothing to do here.")
		return
	}
	i.logger.Debug("Lock Acquired", zap.String("key", lockKey))
	// TODOs: We need to inform the controller here! We will create a API in controller and call
	// It from here
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
