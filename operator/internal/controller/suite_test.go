/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
	"truefoundry/elasti/operator/internal/crddirectory"
	"truefoundry/elasti/operator/internal/informer"

	uberZap "go.uber.org/zap"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	ctrl "sigs.k8s.io/controller-runtime"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	elastiv1alpha1 "truefoundry/elasti/operator/api/v1alpha1"
)

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var namespace = "elasti-test"

var mgrCtx context.Context
var mgrCancel context.CancelFunc
var informerManager *informer.Manager
var controllerReconciler *ElastiServiceReconciler

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Suite", Label("controller"))
}

var _ = BeforeSuite(func() {
	zapLogger := zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
	logf.SetLogger(zapLogger)

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:       []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing:   true,
		ControlPlaneStopTimeout: 1 * time.Minute,
		BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s",
			fmt.Sprintf("1.29.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = elastiv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
	err = k8sClient.Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	})
	Expect(err).NotTo(HaveOccurred())

	informerManager = informer.NewInformerManager(uberZap.NewExample(), cfg)
	informerManager.Start()
	crddirectory.InitDirectory(uberZap.NewExample())

	mgrCtx, mgrCancel = context.WithCancel(context.Background())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: k8sClient.Scheme(),
	})
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = mgr.Start(mgrCtx)
		Expect(err).NotTo(HaveOccurred())
		gexec.KillAndWait(5 * time.Second)

		err := testEnv.Stop()
		Expect(err).NotTo(HaveOccurred())
	}()

	controllerReconciler = &ElastiServiceReconciler{
		Client:             k8sClient,
		Scheme:             k8sClient.Scheme(),
		Logger:             uberZap.NewExample(),
		InformerManager:    informerManager,
		SwitchModeLocks:    sync.Map{},
		InformerStartLocks: sync.Map{},
		ReconcileLocks:     sync.Map{},
	}
	Expect(controllerReconciler.SetupWithManager(mgr)).To(Succeed())
})

var _ = AfterSuite(func() {
	informerManager.Stop()
	mgrCancel()
})
