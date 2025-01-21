package crddirectory

import (
	"sync"

	"truefoundry/elasti/operator/api/v1alpha1"

	"go.uber.org/zap"
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

func InitDirectory(logger *zap.Logger) {
	directoryMutexOnce.Do(func() {
		CRDDirectory = &Directory{
			Logger: logger.Named("CRDDirectory"),
		}
	})
}

func AddCRD(serviceName string, crdDetails *CRDDetails) {
	CRDDirectory.Services.Store(serviceName, crdDetails)
}

func RemoveCRD(serviceName string) {
	CRDDirectory.Services.Delete(serviceName)
}

func GetCRD(serviceName string) (*CRDDetails, bool) {
	value, ok := CRDDirectory.Services.Load(serviceName)
	if !ok {
		return nil, false
	}
	CRDDirectory.Logger.Info("Service found in directory", zap.String("service", serviceName))
	return value.(*CRDDetails), true
}
