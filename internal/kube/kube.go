package kube

import (
	"context"
	"os"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type ClientType int

// This function is from the Kubernetes code base, I did not see any
// easy way to grab this info without changing code flow of how the
// client is provisioned, so for now I have just copied it
//
// https://github.com/kubernetes/kubernetes/blob/8e6d788887034b799f6c2a86991a68a080bb0576/staging/src/k8s.io/client-go/tools/clientcmd/client_config.go#L631
func getNamespaceForInClusterConfig() string {
	// This way assumes you've set the POD_NAMESPACE environment variable using the downward API.
	// This check has to be done first for backwards compatibility with the way InClusterConfig was originally set up
	if ns, ok := os.LookupEnv("POD_NAMESPACE"); ok {
		return ns
	}

	// Fall back to the namespace associated with the service account token, if available
	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	}

	return "default"
}

const (
	InCluster ClientType = iota
	OutOfCluster
)

type KubeClient struct {
	ClientType ClientType
	Client     *kubernetes.Clientset
	Namespace  string
	Config     *rest.Config
}

func buildOutOfClusterConfig(kubeconfigOverride string, contextOverride string) (*rest.Config, string, error) {
	var kubeconfigPath string
	if kubeconfigOverride != "" {
		kubeconfigPath = kubeconfigOverride
	} else {
		kubeconfigPath = os.Getenv("KUBECONFIG")
		if kubeconfigPath == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil, "", err
			}
			kubeconfigPath = homeDir + "/.kube/config"
		}
	}

	clientcmdConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{CurrentContext: contextOverride},
	)

	currentNamespace, _, err := clientcmdConfig.Namespace()
	if err != nil {
		return nil, "", err
	}

	clientConfig, err := clientcmdConfig.ClientConfig()
	if err != nil {
		return nil, "", err
	}
	return clientConfig, currentNamespace, nil
}

func GetClient(kubeconfigOverride string, context string, namespaceOverride string) (*KubeClient, error) {
	clientType := InCluster
	kcfg, err := rest.InClusterConfig()
	var namespace string

	if err != nil {
		clientType = OutOfCluster
		kcfg, namespace, err = buildOutOfClusterConfig(kubeconfigOverride, context)
		if err != nil {
			return nil, err
		}
	} else {
		namespace = getNamespaceForInClusterConfig()
	}

	if namespaceOverride != "" {
		namespace = namespaceOverride
	}

	if namespace == "" {
		namespace = "default"
	}

	client, err := kubernetes.NewForConfig(kcfg)
	if err != nil {
		return nil, err
	}

	return &KubeClient{
		ClientType: clientType,
		Client:     client,
		Namespace:  namespace,
		Config:     kcfg,
	}, nil
}

func GetPodsForService(ctx context.Context, kubeClient *KubeClient, serviceName *string) (*corev1.PodList, *corev1.Service, error) {
	service, err := kubeClient.Client.CoreV1().Services(kubeClient.Namespace).Get(ctx, *serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	set := labels.Set(service.Spec.Selector)
	listOptions := metav1.ListOptions{LabelSelector: set.AsSelector().String()}
	pods, err := kubeClient.Client.CoreV1().Pods(kubeClient.Namespace).List(ctx, listOptions)
	if err != nil {
		return nil, nil, err
	}
	return pods, service, err
}

func GetClientUsingFlags(command *cobra.Command) (*KubeClient, error) {
	kubeconfig, err := command.Flags().GetString("kubeconfig")
	if err != nil {
		return nil, err
	}
	context, err := command.Flags().GetString("context")
	if err != nil {
		return nil, err
	}
	namespace, err := command.Flags().GetString("namespace")
	if err != nil {
		return nil, err
	}
	return GetClient(kubeconfig, context, namespace)
}
