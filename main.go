package main

import (
	"log"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// RegisteelEndpoint determines what api endpoint we need to submit
// new services too
var RegisteelEndpoint string

func init() {
	RegisteelEndpoint = GetEnv("REGISTEEL_API_ADDRESS", "localhost:8090/deployments")
}

// main provides the entry point to the controller, this sets up
// the client and connections to the kubernetes api and starts
// the controller.
func main() {
	clientset, err := NewClientSet()
	if err != nil {
		log.Fatalf("clientset failed to load: %v", err)
	}

	c := NewController(clientset)

	stop := make(chan struct{})
	defer close(stop)

	if err = c.Run(1, stop); err != nil {
		log.Fatalf("Error running controller: %s", err.Error())
	}
}

// NewClientSet sets up the environment to connect to the Kubernetes cluster
// this will use the context you are currently defaulted too, so be careful
// and don't point at a cluster that is in production ;)
func NewClientSet() (kubernetes.Interface, error) {
	lr := clientcmd.NewDefaultClientConfigLoadingRules()
	kc := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(lr, &clientcmd.ConfigOverrides{})

	config, err := kc.ClientConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

// GetEnv is a helper function that helps us set the api address
func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
