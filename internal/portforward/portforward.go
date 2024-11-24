package portforward

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type PortForwarder struct {
	clientset  *kubernetes.Clientset
	config     *rest.Config
	namespace  string
	podName    string
	localPort  int
	remotePort int
	stopChan   chan struct{}
	readyChan  chan struct{}
	errChan    chan error
}

func NewPortForwarder(namespace, service string, localPort, remotePort int) (*PortForwarder, error) {
	// Get kubernetes config
	config, err := getKubeConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Get pod name from service
	pod, err := getPodFromService(clientset, namespace, service)
	if err != nil {
		return nil, err // Return the error directly since getPodFromService now returns proper errors
	}

	if pod == nil {
		return nil, fmt.Errorf("no running pods found for service %s in namespace %s", service, namespace)
	}

	return &PortForwarder{
		clientset:  clientset,
		config:     config,
		namespace:  namespace,
		podName:    pod.Name,
		localPort:  localPort,
		remotePort: remotePort,
		stopChan:   make(chan struct{}, 1),
		readyChan:  make(chan struct{}, 1),
		errChan:    make(chan error, 1),
	}, nil
}

func (pf *PortForwarder) Start(ctx context.Context) error {
	fmt.Printf("Starting port forward to pod %s/%s (%d -> %d)\n",
		pf.namespace, pf.podName, pf.localPort, pf.remotePort)

	// Create request URL
	req := pf.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(pf.namespace).
		Name(pf.podName).
		SubResource("portforward")

	// Use stored config instead of trying to get it from RESTClient
	transport, upgrader, err := spdy.RoundTripperFor(pf.config)
	if err != nil {
		return fmt.Errorf("failed to create round tripper: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
	ports := []string{fmt.Sprintf("%d:%d", pf.localPort, pf.remotePort)}

	// Create error and output buffers for debugging
	errorBuffer := &bytes.Buffer{}
	outputBuffer := &bytes.Buffer{}

	// Start port forwarding
	forwarder, err := portforward.New(dialer, ports, pf.stopChan, pf.readyChan, outputBuffer, errorBuffer)
	if err != nil {
		return fmt.Errorf("failed to create port forwarder: %w", err)
	}

	go func() {
		if err := forwarder.ForwardPorts(); err != nil {
			pf.errChan <- fmt.Errorf("port forward failed: %w (error buffer: %s)", err, errorBuffer.String())
		}
	}()

	// Wait for ready or error with better feedback
	select {
	case <-pf.readyChan:
		fmt.Println("Port forward ready")
		return nil
	case err := <-pf.errChan:
		return fmt.Errorf("port forward failed: %w\nOutput: %s\nError: %s",
			err, outputBuffer.String(), errorBuffer.String())
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for port forward\nOutput: %s\nError: %s",
			outputBuffer.String(), errorBuffer.String())
	}
}

func (pf *PortForwarder) Stop() {
	close(pf.stopChan)
}

func (pf *PortForwarder) Ready() <-chan struct{} {
	return pf.readyChan
}

func (pf *PortForwarder) Errors() <-chan error {
	return pf.errChan
}

// Helper functions
func getKubeConfig() (*rest.Config, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fall back to kubeconfig file
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig from %s: %w", kubeconfig, err)
	}
	return config, nil
}

func getPodFromService(clientset *kubernetes.Clientset, namespace, serviceName string) (*v1.Pod, error) {
	// Get service
	svc, err := clientset.CoreV1().Services(namespace).Get(context.Background(), serviceName, metav1.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("service '%s' not found in namespace '%s' - please verify the Kubiya runner is installed and running", serviceName, namespace)
		}
		return nil, fmt.Errorf("failed to get service: %w", err)
	}

	// Try to find pods with service selector
	pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{
			MatchLabels: svc.Spec.Selector,
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// If no pods found, provide a clear error message
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods found for service '%s' in namespace '%s' - please verify the deployment is running", serviceName, namespace)
	}

	// Find first ready pod
	for _, pod := range pods.Items {
		if pod.Status.Phase == v1.PodRunning {
			for _, condition := range pod.Status.Conditions {
				if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
					return &pod, nil
				}
			}
		}
	}

	// If pods exist but none are ready
	return nil, fmt.Errorf("found %d pod(s) for service '%s' but none are ready - please check pod status and logs", len(pods.Items), serviceName)
}

// Update SetContext to store the new config
func (pf *PortForwarder) SetContext(context string) error {
	// Load the kubeconfig
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: context,
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		configOverrides,
	).ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to load context '%s': %w", context, err)
	}

	// Create new clientset with updated config
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client with context '%s': %w", context, err)
	}

	// Update both config and clientset
	pf.config = config
	pf.clientset = clientset
	return nil
}

// WaitUntilReady waits for the port forward to be ready
func (pf *PortForwarder) WaitUntilReady(ctx context.Context) error {
	fmt.Println("Waiting for port forward to be ready...")
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(10 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled while waiting for port forward")
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", pf.localPort), time.Second)
			if err == nil {
				conn.Close()
				fmt.Println("Port forward connection established")
				return nil
			}
		case err := <-pf.errChan:
			return fmt.Errorf("port forward failed while waiting: %w", err)
		case <-timeout:
			return fmt.Errorf("timeout waiting for port forward to be ready")
		}
	}
}
