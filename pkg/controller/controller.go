package controller

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/zduymz/tag-to-label/pkg/apis/tag-to-label"
	"github.com/zduymz/tag-to-label/pkg/provider"
	"github.com/zduymz/tag-to-label/pkg/utils"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

/*
	Run 2 thread:
	+ one listening on Adding Event of Worker node
	+ one checking AWS worker on interval(5m) and update if there is difference.
*/
const TagNamePrefix = "devops.apixio.com/"

type Controller struct {
	nodeLister    corelisters.NodeLister
	kubeclientset kubernetes.Interface
	hasSynced     cache.InformerSynced
	workqueue     workqueue.RateLimitingInterface
	provider      *provider.AWSProvider
}

func NewController(nodeInformer coreinformers.NodeInformer, kubeclientset kubernetes.Interface, config *tag_to_label.Config) (*Controller, error) {
	klog.Info("Setting up AWS")

	p, err := provider.NewAWSProvider(provider.AWSConfig{
		Region:       config.AWSRegion,
		AssumeRole:   config.AWSAssumeRole,
		AWSCredsFile: config.AWSCredsFile,
		APIRetries:   3,
	})
	if err != nil {
		klog.Errorf("Error: %s", err.Error())
		return nil, err
	}

	controller := &Controller{
		nodeLister:    nodeInformer.Lister(),
		hasSynced:     nodeInformer.Informer().HasSynced,
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Worker Tag"),
		provider:      p,
		kubeclientset: kubeclientset,
	}

	klog.Info("Setting up event handlers")

	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleAddObject,
		//UpdateFunc: func(old, new interface{}) {
		//	newOne := new.(*corev1.Pod)
		//	oldOne := old.(*corev1.Pod)
		//	if newOne.ResourceVersion == oldOne.ResourceVersion {
		//		return
		//	}
		//	controller.handleAddObject(new)
		//},
		//DeleteFunc: controller.handleDeleteObject,
	})

	return controller, nil
}

// Run will set event handler for pod, syncing informer caches and starting workers.
func (c *Controller) Run(stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.Info("[main] Starting controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("[main] Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.hasSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("[main] Starting worker")
	go wait.Until(c.runWorker, time.Second, stopCh)
	klog.Info("[main] Started  worker ")
	klog.Info("[main] Starting checker")
	go wait.Until(c.runChecker, 5*time.Minute, stopCh)
	klog.Info("[main] Started  checker ")

	<-stopCh
	klog.Info("[main] Shutting down worker and checker")
	return nil
}

// Query aws instances to get tags
func (c *Controller) runChecker() {
	klog.Info("[runChecker] Start runChecker")
	nodes, err := c.nodeLister.List(labels.NewSelector())
	if err != nil {
		klog.Errorf("[runChecker] Failed to list nodes. Reason: %s", err.Error())
		return
	}
	var instanceIds []*string
	nodeNameById := map[string]string{}
	for _, no := range nodes {
		// ProviderID: aws:///us-west-2c/i-08aab319ad2b55083
		id, _ := utils.LastinSlice(strings.Split(no.Spec.ProviderID, "/"))
		instanceIds = append(instanceIds, aws.String(id))
		nodeNameById[id] = no.GetName()
	}

	klog.Info("[runChecker] Show all instances ids")
	tagsById, err := c.provider.ListTags(instanceIds)
	if err != nil {
		klog.Errorf("[runChecker] Can not list aws tag. Reason: %s", err.Error())
		return
	}

	for id, tags := range FilterTag(tagsById) {
		klog.Infof("[runChecker] Checking instance id: %s", id)
		trimmedTags := TrimTag(tags)
		no, err := c.nodeLister.Get(nodeNameById[id])
		if err != nil {
			klog.Errorf("[runChecker] %s", err.Error())
			continue
		}
		updateLabels, _ := OuterRightJoin(no.Labels, trimmedTags)
		if len(updateLabels) > 0 {
			klog.Infof("[runChecker] Updating Labels on node [%s]", no.GetName())
			err := c.updateNodeLabels(no.GetName(), updateLabels)
			if err != nil {
				klog.Errorf("[runChecker] Can not update labels on node [%s]", no.GetName())
			}
		}
	}
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			klog.Errorf("[worker] expected string in workqueue but got %#v", obj)
			return nil
		}
		klog.V(4).Infof("[worker] syncHanlder worker %s", key)
		if err := c.syncHandler(key); err != nil {
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("[worker] Error syncing '%s': %s, Requeuing", key, err.Error())
		}

		c.workqueue.Forget(obj)
		klog.Infof("[worker] Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) syncHandler(key string) error {
	no, err := c.nodeLister.Get(key)
	if err != nil {
		return err
	}

	//TODO: should update labels in other states
	if !c.isNodeRunning(no) {
		return fmt.Errorf("node [%s] is not running", no.GetName())
	}

	id, _ := utils.LastinSlice(strings.Split(no.Spec.ProviderID, "/"))
	tags, err := c.provider.ListTags([]*string{aws.String(id)})
	if err != nil {
		klog.Errorf("[worker] Can not list aws tag")
		return err
	}
	klog.V(4).Info("[worker] Raw tags: ", tags)
	// Is it throw error
	filteredTags := TrimTag(FilterTag(tags)[id])

	klog.Info("Filtered tags: ", filteredTags)

	updateLabels, _ := OuterRightJoin(no.Labels, filteredTags)
	if len(updateLabels) > 0 {
		klog.Infof("[worker] Updating Labels on node [%s]", no.GetName())
		//nodeCopy := no.DeepCopy()
		//for k, v := range updateLabels {
		//	nodeCopy.Labels[k] = v
		//}
		//_, err := c.kubeclientset.CoreV1().Nodes().Update(nodeCopy)
		//if err != nil {
		//	klog.Errorf("Can not update labels")
		//	return err
		//}
		err := c.updateNodeLabels(no.GetName(), updateLabels)
		if err != nil {
			klog.Errorf("[worker] Can not update labels on node [%s]", no.GetName())
			return err
		}
	}

	return nil
}

//TODO: no idea why panic happen when calling this function with signature
// func (c *Controller) updateNodeLabels(no *corev1.Node, newLabels map[string]string) error {
func (c *Controller) updateNodeLabels(nodeName string, newLabels map[string]string) error {
	no, _ := c.nodeLister.Get(nodeName)
	nodeCopy := no.DeepCopy()
	for k, v := range newLabels {
		nodeCopy.Labels[k] = v
	}
	// TODO: is it a good to update directly?
	_, err := c.kubeclientset.CoreV1().Nodes().Update(nodeCopy)
	return err
}

// TODO: this step is quite redundant, what tombstone is?
func (c *Controller) handleAddObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("error decoding object, invalid type")
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			klog.Errorf("error decoding object tombstone, invalid type")
			return
		}
		klog.Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}

	klog.V(4).Infof("Processing object: %s", object.GetName())

	// TODO: should we check object KIND before converting?
	no, err := c.nodeLister.Get(object.GetName())
	if err != nil {
		klog.Infof("Can not get node [%s] ", object.GetName())
		return
	}
	c.workqueue.Add(no.Name)
}

func (c *Controller) isNodeRunning(no *corev1.Node) bool {
	for _, cond := range no.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return true
		}
	}
	return false
}

func OuterRightJoin(left, right map[string]string) (map[string]string, error) {
	output := make(map[string]string)
	for key, value := range right {
		if _, exist := left[key]; exist {
			// exist but value is different
			if left[key] != value {
				output[key] = value
			}
		} else {
			output[key] = value
		}
	}
	return output, nil
}

func FilterTag(tags map[string][]*provider.Tag) map[string][]*provider.Tag {
	result := make(map[string][]*provider.Tag)
	for instanceId, instanceTags := range tags {
		result[instanceId] = make([]*provider.Tag, 0)
		for _, tag := range instanceTags {
			if strings.HasPrefix(tag.Key, TagNamePrefix) {
				result[instanceId] = append(result[instanceId], tag)
			}
		}
	}
	return result
}

func TrimTag(tags []*provider.Tag) map[string]string {
	result := map[string]string{}
	for _, tag := range tags {
		if strings.HasPrefix(tag.Key, TagNamePrefix) {
			key := strings.TrimPrefix(tag.Key, TagNamePrefix)
			result[key] = tag.Value
		}
	}
	return result
}
