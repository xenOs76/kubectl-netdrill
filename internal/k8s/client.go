package k8s

import (
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ClientProvider defines a function that returns a Kubernetes client and rest config.
var ClientProvider = GetClient

// GetClient returns a Kubernetes clientset using the provided config flags.
func GetClient(configFlags *genericclioptions.ConfigFlags) (kubernetes.Interface, *rest.Config, error) {
	config, err := configFlags.ToRESTConfig()
	if err != nil {
		return nil, nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	return clientset, config, nil
}
