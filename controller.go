package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	appsv1 "k8s.io/api/apps/v1"
	listers "k8s.io/client-go/listers/apps/v1"
)

// Payload is the interface we use between the API and our Kubernetes
// objectes. Using this structure provides a way for us to diff against
// the api and our current state.
type Payload struct {
	ID        types.UID         `json:"id"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"label"`
}

// Controller is the controller implementation for Registeel
type Controller struct {
	client   kubernetes.Interface
	informer cache.SharedIndexInformer
	lister   listers.DeploymentLister
	logger   *logrus.Logger
	queue    workqueue.RateLimitingInterface
}

// NewController builds and returns a simple controller that
// uses a shared informer to watch deployments and respond to them on
// change, update and delete.
func NewController(client kubernetes.Interface) *Controller {
	shared := informers.NewSharedInformerFactory(client, time.Second*30)
	inform := shared.Apps().V1().Deployments()
	contrl := &Controller{
		client:   client,
		informer: inform.Informer(),
		lister:   inform.Lister(),
		logger:   logrus.New(),
		queue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "regitseel"),
	}

	inform.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: contrl.enqueue,
			UpdateFunc: func(old, new interface{}) {
				contrl.enqueue(new)
			},
			DeleteFunc: func(obj interface{}) {
				d := obj.(*appsv1.Deployment)
				if err := contrl.delete(d); err != nil {
					contrl.logger.Errorf("failed to delete from api: %v", d.Name)
				}
			},
		},
	)

	return contrl
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()

	c.logger.Info("Starting registeel controller")
	c.logger.Infof("Api endpoint registered as: %v", RegisteelEndpoint)

	go c.informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		return fmt.Errorf("Timed out waiting for cache to sync")
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh

	return nil
}

// enqueue takes a Deployment resource and converts it into a namespace/name
// string which is then put onto the work queue.
func (c *Controller) enqueue(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}

	c.queue.AddRateLimited(key)
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	// Wait until there is a new item in the working queue
	obj, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.queue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.queue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.queue.AddRateLimited(key)
			//return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.queue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Deployment resource with this namespace/name
	d, err := c.lister.Deployments(namespace).Get(name)
	if err != nil {
		// If the resource does not exist, we need to update the API
		if errors.IsNotFound(err) {
			return nil
		}

		return err
	}

	// Sync resources in API
	err = c.sync(d)
	if err != nil {
		return err
	}

	return nil
}

// update will query the api and see if the version in the api is different
// then the current version in queue and attempt to make the state of those
// the same
func (c *Controller) sync(d *appsv1.Deployment) error {
	// contact api to get current version
	ep := RegisteelEndpoint + "/" + string(d.UID)
	res, err := http.Get(ep)
	if err != nil {
		return fmt.Errorf("failed to retrieve status from api: %v", d.Name)
	}

	temp, _ := ioutil.ReadAll(res.Body)

	// unmarshal payload into a response object we can use for comparison
	var apiResponse Payload
	err = json.Unmarshal(temp, &apiResponse)
	if err != nil {
		return fmt.Errorf("Error unmarshaling from api: %v", err)
	}

	// build response object of newest version
	pl := &Payload{
		ID:        d.UID,
		Name:      d.Name,
		Namespace: d.Namespace,
		Labels:    d.GetLabels(),
	}

	// if the payloads are equal, log and skip
	if cmp.Equal(&apiResponse, pl) {
		c.logger.Println("api is already up to date for: ", pl.Name)
		return nil
	}

	// update api with new payload
	c.logger.Infof("api out of sync for %v", d.Name)
	if err := c.submitToAPI(pl); err == nil {
		// once api has been updated, then annotate resources
		if err := c.annotateDeployment(d); err != nil {
			return fmt.Errorf("error annotating deployment: %v", err)
		}

		c.logger.Infof("api has been updated for %v", d.Name)
	} else {
		return fmt.Errorf("error updating api, annotations not applied: %v", pl.Name)
	}

	return nil
}

// deleteInRegistry will respond to a deleted deployment and inform the registry
// that the application no longer exists
func (c *Controller) delete(d *appsv1.Deployment) error {
	ep := RegisteelEndpoint + "/" + string(d.UID)
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodDelete, ep, nil)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	if err != nil {
		return err
	}

	_, err = client.Do(req)
	if err != nil {
		return err
	}

	c.logger.Infof("removed deployment from api: %v", d.Name)

	return nil
}

// existsInAPI determines if the resource is already present in the api
// so that we can either create a new version or update the version
// already present
func (c *Controller) existsInAPI(pl *Payload) (bool, error) {
	ep := RegisteelEndpoint + "/" + string(pl.ID)
	res, err := http.Get(ep)
	if err != nil {
		return false, fmt.Errorf("failed to retrieve status from api: %v", pl.Name)
	}

	if res.StatusCode == 200 {
		return true, nil
	}

	return false, nil
}

// submitToAPI will send an updated payload to the api endpoint
// and then annotate the deployment
func (c *Controller) submitToAPI(pl *Payload) error {
	e, err := json.Marshal(pl)
	if err != nil {
		return err
	}

	// see if resource already exists
	exists, err := c.existsInAPI(pl)
	if err != nil {
		return err
	}

	// attempt to create the resource
	if !exists {
		if err := c.createInAPI(RegisteelEndpoint, bytes.NewBuffer(e)); err != nil {
			return err
		}
	}

	// attempt to update the resource
	uri := RegisteelEndpoint + "/" + string(pl.ID)
	if err := c.updateInAPI(uri, bytes.NewBuffer(e)); err != nil {
		return err
	}

	return nil
}

// createInAPI will post the request to the proper endpoint
func (c *Controller) createInAPI(uri string, data io.Reader) error {
	resp, err := http.Post(uri, "application/json; charset=utf-8", data)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusCreated {
		log.Printf("status code received back was not expected during creation")
	}

	return nil
}

// updateInAPI will make a patch request to the proper endpoint
func (c *Controller) updateInAPI(uri string, data io.Reader) error {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPatch, uri, data)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	if err != nil {
		return err
	}

	_, err = client.Do(req)
	if err != nil {
		return err
	}

	return nil
}

// annotateDeployment will set the proper annotations once the API has been
// updated
func (c *Controller) annotateDeployment(d *appsv1.Deployment) error {
	t := time.Now()
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	copy := d.DeepCopy()

	copy.SetAnnotations(map[string]string{
		"app.registeel.io/registered":   "true",
		"app.registeel.io/last-updated": t.Format(time.RFC3339),
		"app.registeel.io/last-version": d.ObjectMeta.ResourceVersion,
	})

	_, err := c.client.AppsV1().Deployments(d.Namespace).Update(d)
	if err != nil {
		return err
	}

	return nil
}
