package crddirectory

import (
	"sync"

	"github.com/truefoundry/elasti/operator/api/v1alpha1"

	"go.uber.org/zap"
)

type Directory struct {
	Services sync.Map
	Logger   *zap.Logger
}

type CRDDetails struct {
	CRDName string
	Spec    v1alpha1.ElastiServiceSpec
	Status  v1alpha1.ElastiServiceStatus
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
	if CRDDirectory == nil {
		panic("CRDDirectory not initialized")
	}
	CRDDirectory.Services.Store(serviceName, crdDetails)
}

func RemoveCRD(serviceName string) {
	if CRDDirectory == nil {
		panic("CRDDirectory not initialized")
	}
	CRDDirectory.Services.Delete(serviceName)
}

func GetCRD(serviceName string) (*CRDDetails, bool) {
	if CRDDirectory == nil {
		panic("CRDDirectory not initialized")
	}
	value, ok := CRDDirectory.Services.Load(serviceName)
	if !ok {
		return nil, false
	}
	CRDDirectory.Logger.Info("Service found in directory", zap.String("service", serviceName))
	return value.(*CRDDetails), true
}
