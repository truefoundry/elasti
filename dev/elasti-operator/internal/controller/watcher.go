package controller

import (
	"context"
	"sync"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"truefoundry.io/elasti/api/v1alpha1"

	"k8s.io/client-go/informers"
)

type WatcherType struct {
	client *kubernetes.Clientset
	logger *zap.Logger
	//hpaWatchList        map[string]bool
	deploymentWatchList sync.Map
	informerFactory     informers.SharedInformerFactory
}

func NewWatcher(logger *zap.Logger, kConfig *rest.Config) *WatcherType {
	clientset, cerr := kubernetes.NewForConfig(kConfig)
	if cerr != nil {
		logger.Fatal("Error connecting with kubernetes", zap.Error(cerr))
	}
	return &WatcherType{
		client: clientset,
		logger: logger.With(zap.String("component", "watcher")),
		//hpaWatchList:        make(map[string]bool),
		deploymentWatchList: sync.Map{},
		informerFactory:     informers.NewSharedInformerFactory(clientset, 0),
	}
}

func (hw *WatcherType) AddAndRunDeploymentWatch(deploymentName, namespace string, es *v1alpha1.ElastiService, runReconcile RunReconcileFunc) {
	if enabled, ok := hw.deploymentWatchList.Load(deploymentName); ok {
		if enabled.(bool) {
			return
		}
	}
	reconcileChan := make(chan string)
	go hw.StartDeploymentWatch(deploymentName, namespace, reconcileChan)
	for mode := range reconcileChan {
		hw.logger.Debug("Reconciling", zap.String("mode", mode))
		runReconcile(context.Background(), ctrl.Request{}, es, mode)
	}
}

func (hw *WatcherType) StartDeploymentWatch(deploymentName, namespace string, updateMode chan<- string) {
	defer func() {
		if r := recover(); r != nil {
			hw.logger.Error("Recovered from panic", zap.Any("error", r))
			hw.deploymentWatchList.Store(deploymentName, false)
			go hw.StartDeploymentWatch(deploymentName, namespace, updateMode)
		}
	}()
	hw.logger.Info("Adding Deployment watch", zap.String("deployment_name", deploymentName))
	informer := hw.informerFactory.Apps().V1().Deployments().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			newDeployment := newObj.(*appsv1.Deployment)
			condition := newDeployment.Status.Conditions
			if newDeployment.Status.Replicas == 0 {
				hw.logger.Debug("Deployment has 0 replicas", zap.String("deployment_name", deploymentName))
				updateMode <- ProxyMode
			} else if newDeployment.Status.Replicas > 0 && condition[1].Status == "True" {
				hw.logger.Debug("Deployment has replicas", zap.String("deployment_name", deploymentName))
				updateMode <- ServeMode
			}
		},
	})

	stop := make(chan struct{})
	defer close(stop)
	go informer.Run(stop)
	hw.deploymentWatchList.Store(deploymentName, true)
	select {}
}

/*
func (hw *WatcherType) AddHPAWatch(hpaName, namespace string) {
	if enabled, ok := hw.hpaWatchList[hpaName]; ok {
		if enabled {
			return
		}
	}
	//go hw.RunHPAWatchV3("app=target", namespace)
	go hw.RunHPAWatch(hpaName, namespace)
	// TODO: Find a better place for this, as it is set even running watch fails
	//hw.hpaWatchList[hpaName] = true
}

func (hw *WatcherType) RunHPAWatchV3(selector, namespace string) {
	watcher, err := hw.client.AppsV1().Deployments(namespace).Watch(context.Background(), metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		panic(err)
	}
	hw.logger.Info("Watching deployments", zap.String("selector", selector), zap.String("namespace", namespace))
	defer watcher.Stop()

	// Handle events from the watch channel
	for event := range watcher.ResultChan() {
		// Extract relevant information from the event
		// For example, you can access the current replicas of the Deployment
		// by inspecting the event object and taking appropriate actions
		hw.logger.Info("Event received", zap.Any("event", event))
	}

}

// This works, but slow
func (hw *WatcherType) RunHPAWatchV2(hpaName, namespace string) {
	for {
		// Query the API server for the HPA of the target service
		hpa, err := hw.client.AutoscalingV1().HorizontalPodAutoscalers(namespace).Get(context.TODO(), hpaName, v1.GetOptions{})
		if err != nil {
			fmt.Println("Error getting HPA:", err)
			continue
		}

		// Check if the HPA has changed
		// You can compare the current state with the previous state to detect changes
		// For example, compare hpa.Status.CurrentReplicas with the previous value

		// Print the current HPA status
		fmt.Printf("Current replicas for %s HPA: %d\n", hpaName, hpa.Status.CurrentReplicas)

		// Sleep for a period before checking again
		time.Sleep(2 * time.Second)
	}
}

// This works, but slow
func (hw *WatcherType) RunHPAWatch(hpaName, namespace string) {
	hw.logger.Info("Adding HPA watch", zap.String("hpaName", hpaName))
	fieldSelector := fields.OneTermEqualSelector("metadata.name", hpaName).String()
	hpaListWatcher := cache.NewListWatchFromClient(
		hw.client.AutoscalingV1().RESTClient(),
		"horizontalpodautoscalers",
		namespace,
		fields.ParseSelectorOrDie(fieldSelector))
	informer := cache.NewSharedInformer(
		hpaListWatcher,
		&autoscalingv1.HorizontalPodAutoscaler{},
		1*time.Second)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			hpa := obj.(*autoscalingv1.HorizontalPodAutoscaler)
			if hpa.Name == hpaName {
				hw.checkHPAReplicas(hpa)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			hpa := newObj.(*autoscalingv1.HorizontalPodAutoscaler)
			if hpa.Name == hpaName {
				hw.checkHPAReplicas(hpa)
			}
		},
		DeleteFunc: func(obj interface{}) {
			hpa := obj.(*autoscalingv1.HorizontalPodAutoscaler)
			if hpa.Name == hpaName {
				hw.checkHPAReplicas(hpa)
			}
		},
	})
	stop := make(chan struct{})
	defer close(stop)
	go informer.Run(stop)
	hw.hpaWatchList[hpaName] = true
	select {}
}

func (hw *WatcherType) checkHPAReplicas(hpa *autoscalingv1.HorizontalPodAutoscaler) {
	hw.logger.Debug("Checking HPA replicas", zap.String("hpaName", hpa.Name))
	// Check if the HPA has the desired number of replicas
	if hpa.Status.CurrentReplicas == 0 {
		//TODO: We need to switch to proxy mode
		hw.logger.Info("HPA has 0 replicas", zap.String("hpaName", hpa.Name))
	}
}
*/
