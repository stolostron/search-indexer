// Copyright Contributors to the Open Cluster Management project

package config

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

// If env KUBECONFIG is defined, use it. Otherise use default location ~/.kube/config
// NOTE: This may need to be enhanced to support development on different OS.
func getKubeConfigPath() string {
	defaultKubePath := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	if _, err := os.Stat(defaultKubePath); os.IsNotExist(err) {
		// set default to empty string if path does not reslove
		defaultKubePath = ""
	}

	kubeConfig := getEnv("KUBECONFIG", defaultKubePath)
	return kubeConfig
}

func getKubeConfig() *rest.Config {
	kubeConfigPath := getKubeConfigPath()
	var clientConfig *rest.Config
	var clientConfigError error

	if kubeConfigPath != "" {
		klog.Infof("Creating k8s client using KubeConfig at: %s", kubeConfigPath)
		clientConfig, clientConfigError = clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	} else {
		klog.V(2).Info("Creating k8s client using InClusterClientConfig")
		clientConfig, clientConfigError = rest.InClusterConfig()
	}

	if clientConfigError != nil {
		klog.Fatal("Error getting Kube Config: ", clientConfigError)
	}

	return clientConfig
}

func getKubeClient() *kubernetes.Clientset {
	config := getKubeConfig()
	var kubeClient *kubernetes.Clientset
	var err error
	if config != nil {
		kubeClient, err = kubernetes.NewForConfig(config)
		if err != nil {
			klog.Fatal("Cannot Construct Kube Client from Config: ", err)
		}
	} else {
		klog.Error("Cannot Construct Kube Client as input Config is nil")
	}
	return kubeClient
}

// Get the kubernetes dynamic client.
func GetDynamicClient() dynamic.Interface {
	newDynamicClient, err := dynamic.NewForConfig(getKubeConfig())
	if err != nil {
		klog.Fatal("Cannot Construct Dynamic Client ", err)
	}

	return newDynamicClient
}
