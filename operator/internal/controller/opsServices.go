package controller

import (
	"context"

	"github.com/truefoundry/elasti/pkg/utils"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"truefoundry.io/elasti/api/v1alpha1"
)

func (r *ElastiServiceReconciler) deletePrivateService(ctx context.Context, publichServiceNamespacedName types.NamespacedName) (err error) {
	privateServiceNamespacedName := publichServiceNamespacedName
	privateServiceNamespacedName.Name = utils.GetPrivateSerivceName(publichServiceNamespacedName.Name)
	privateSVC := &v1.Service{}
	if err := r.Get(ctx, privateServiceNamespacedName, privateSVC); err != nil {
		if errors.IsNotFound(err) {
			r.Logger.Info("Private Service already deleted or not found", zap.String("private-service", privateServiceNamespacedName.String()))
			return nil
		}
		return err
	}
	if err := r.Delete(ctx, privateSVC); err != nil {
		return err
	}
	return nil
}

func (r *ElastiServiceReconciler) checkAndCreatePrivateService(ctx context.Context, publicSVC *v1.Service, es *v1alpha1.ElastiService) (PVTName string, err error) {
	PVTName = utils.GetPrivateSerivceName(publicSVC.Name)
	privateServiceNamespacedName := types.NamespacedName{Name: PVTName, Namespace: publicSVC.Namespace}
	// See if private service already exist
	privateSVC := &v1.Service{}
	if err := r.Get(ctx, privateServiceNamespacedName, privateSVC); err != nil && !errors.IsNotFound(err) {
		r.Logger.Error("Failed to get private service", zap.Error(err))
	} else if errors.IsNotFound(err) {
		r.Logger.Info("Private service not found, creating one", zap.String("private-service", privateServiceNamespacedName.String()))
	} else {
		r.Logger.Info("Private service already exists", zap.String("private-service", privateServiceNamespacedName.String()))
		return PVTName, nil
	}

	ports := []v1.ServicePort{}

	for _, port := range publicSVC.Spec.Ports {
		ports = append(ports, v1.ServicePort{
			Name:       port.Name,
			Protocol:   port.Protocol,
			Port:       port.Port,
			TargetPort: port.TargetPort,
		})
	}

	privateSVC = &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PVTName,
			Namespace: publicSVC.Namespace,
		},
		Spec: v1.ServiceSpec{
			Selector: publicSVC.Spec.Selector,
			Ports:    ports,
			Type:     v1.ServiceTypeClusterIP,
		},
	}

	if err := controllerutil.SetControllerReference(es, privateSVC, r.Scheme); err != nil {
		return PVTName, err
	}
	err = r.Create(ctx, privateSVC)
	if err != nil {
		r.Logger.Error("Failed to create private service", zap.String("private-service", privateServiceNamespacedName.String()), zap.Error(err))
		return PVTName, err
	}
	return PVTName, nil
}
