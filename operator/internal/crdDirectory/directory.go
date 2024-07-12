package crdDirectory

import (
	"sync"

	"go.uber.org/zap"
	"truefoundry.io/elasti/api/v1alpha1"
)

type Directory struct {
	Services sync.Map
	Logger   *zap.Logger
}

type CRDDetails struct {
	CRDName string
	Spec    v1alpha1.ElastiServiceSpec
}

var CRDDirectory *Directory

var directoryMutexOnce sync.Once

func INITDirectory(logger *zap.Logger) {
	directoryMutexOnce.Do(func() {
		CRDDirectory = &Directory{
			Logger: logger.Named("CRDDirectory"),
		}
	})
}

func (d *Directory) AddCRD(serviceName string, crdDetails *CRDDetails) {
	d.Services.Store(serviceName, crdDetails)
}

func (d *Directory) RemoveCRD(serviceName string) {
	d.Services.Delete(serviceName)
}

func (d *Directory) GetCRD(serviceName string) (*CRDDetails, bool) {
	value, ok := d.Services.Load(serviceName)
	if !ok {
		return nil, false
	}
	d.Logger.Info("Service found in directory", zap.String("service", serviceName))
	return value.(*CRDDetails), true
}
