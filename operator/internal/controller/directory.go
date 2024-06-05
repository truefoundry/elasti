package controller

import (
	"sync"

	"go.uber.org/zap"
)

type Directory struct {
	Services sync.Map
	Logger   *zap.Logger
}

type CRDDetails struct {
	CRDName        string
	DeploymentName string
}

var ServiceDirectory *Directory

var serviceDirectoryOnce sync.Once

func INITDirectory(logger *zap.Logger) {
	serviceDirectoryOnce.Do(func() {
		ServiceDirectory = &Directory{
			Logger: logger,
		}
	})
}

func (d *Directory) AddService(serviceName string, crdDetails *CRDDetails) {
	d.Services.Store(serviceName, crdDetails)
}

func (d *Directory) GetService(serviceName string) (*CRDDetails, bool) {
	value, ok := d.Services.Load(serviceName)
	if !ok {
		d.Logger.Error("Service not found in directory", zap.String("service", serviceName))
		return nil, false
	}
	d.Logger.Info("Service found in directory", zap.String("service", serviceName))
	return value.(*CRDDetails), true
}
