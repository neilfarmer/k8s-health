package checks

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/neilfarmer/k8s-health/internal/kube"
)

// errControlPlaneNotFound is returned by controlPlanePodHealthz when the
// labelled pod can't be located in the namespace. Callers should turn this
// into a SKIP finding — managed Kubernetes hides these pods.
var errControlPlaneNotFound = errors.New("control-plane pod not found")

// controlPlanePodHealthz finds a pod by label selector in namespace and
// fetches /healthz via the apiserver's pod proxy. scheme is "http" or
// "https"; HTTPS pods need the "https:" prefix in the proxy URL.
func controlPlanePodHealthz(ctx context.Context, env *kube.Env, namespace, selector, scheme string, port int) (string, error) {
	one := int64(1)
	pods, err := env.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
		Limit:         one,
	})
	if err != nil {
		return "", fmt.Errorf("list pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return "", errControlPlaneNotFound
	}
	pod := &pods.Items[0]

	name := fmt.Sprintf("%s:%d", pod.Name, port)
	if scheme == "https" {
		name = "https:" + name
	}
	body, err := env.Clientset.CoreV1().RESTClient().
		Get().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(name).
		SubResource("proxy").
		Suffix("healthz").
		DoRaw(ctx)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", errControlPlaneNotFound
		}
		return "", err
	}
	return string(body), nil
}
