// Package k8s wraps the Kubernetes and Metrics API for resource usage collection.
package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type cpuMem struct{ cpuMillis, memBytes int64 }

type PodUsage struct {
	Namespace string
	Name      string
	OwnerKind string // Deployment, StatefulSet, DaemonSet, or ""
	OwnerName string // owner resource name, or ""
	RequestCPU float64
	RequestMem float64
	ActualCPU  float64
	ActualMem  float64
}

type Client struct {
	clientset *kubernetes.Clientset
	dynamic   dynamic.Interface
}

// InCluster builds a Client from the pod's service account.
func InCluster() (*Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("in-cluster config: %w", err)
	}
	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("kubernetes client: %w", err)
	}
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}
	return &Client{clientset: cs, dynamic: dyn}, nil
}

// ListPods returns all pods across all namespaces.
func (c *Client) ListPods(ctx context.Context) ([]corev1.Pod, error) {
	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	return pods.Items, nil
}

func milliToCPU(m int64) float64  { return float64(m) / 1000 }
func bytesToGB(b float64) float64 { return b / (1024 * 1024 * 1024) }

// podOwner returns the top-level owner kind and name (ReplicaSet → Deployment chain).
func podOwner(pod *corev1.Pod) (kind, name string) {
	for _, ref := range pod.OwnerReferences {
		if ref.Controller != nil && *ref.Controller {
			return ref.Kind, ref.Name
		}
	}
	return "", ""
}

// podRequest extracts the sum of container requests across all containers.
func podRequest(pod *corev1.Pod, resource corev1.ResourceName) float64 {
	var total int64
	for _, c := range pod.Spec.Containers {
		if q, ok := c.Resources.Requests[resource]; ok {
			total += q.MilliValue()
		}
	}
	if resource == corev1.ResourceMemory {
		return float64(total) / (1024 * 1024 * 1024 * 1000)
	}
	return milliToCPU(total)
}

func parseMetricsList(items []unstructured.Unstructured) map[string]cpuMem {
	m := map[string]cpuMem{}
	for _, item := range items {
		key := item.GetNamespace() + "/" + item.GetName()
		containers, found, _ := unstructured.NestedSlice(item.Object, "containers")
		if !found {
			continue
		}
		var cpu, mem int64
		for _, c := range containers {
			cm, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			usage, _ := cm["usage"].(map[string]interface{})
			if cpuStr, ok := usage["cpu"].(string); ok {
				if q, err := parseQuantity(cpuStr); err == nil {
					cpu += q
				}
			}
			if memStr, ok := usage["memory"].(string); ok {
				if q, err := parseQuantity(memStr); err == nil {
					mem += q
				}
			}
		}
		m[key] = cpuMem{cpu, mem}
	}
	return m
}

func (c *Client) ownerFor(ctx context.Context, p *corev1.Pod) (kind, name string) {
	kind, name = podOwner(p)
	if kind == "ReplicaSet" {
		if k, n := c.resolveReplicaSetOwner(ctx, p.Namespace, name); k != "" {
			return k, n
		}
	}
	return kind, name
}

// PodUsages fetches pods and their metrics-server usage in one pass.
func (c *Client) PodUsages(ctx context.Context) ([]PodUsage, error) {
	pods, err := c.ListPods(ctx)
	if err != nil {
		return nil, err
	}

	mres, err := c.dynamic.Resource(schema.GroupVersionResource{
		Group:    "metrics.k8s.io",
		Version:  "v1beta1",
		Resource: "pods",
	}).List(ctx, metav1.ListOptions{})

	usageMap := map[string]cpuMem{}
	if err == nil {
		usageMap = parseMetricsList(mres.Items)
	}

	var result []PodUsage
	for i := range pods {
		p := &pods[i]
		if p.Status.Phase != corev1.PodRunning {
			continue
		}
		u := usageMap[p.Namespace+"/"+p.Name]
		ownerKind, ownerName := c.ownerFor(ctx, p)
		result = append(result, PodUsage{
			Namespace:  p.Namespace,
			Name:       p.Name,
			OwnerKind:  ownerKind,
			OwnerName:  ownerName,
			RequestCPU: podRequest(p, corev1.ResourceCPU),
			RequestMem: podRequest(p, corev1.ResourceMemory),
			ActualCPU:  milliToCPU(u.cpuMillis),
			ActualMem:  bytesToGB(float64(u.memBytes)),
		})
	}
	return result, nil
}

// resolveReplicaSetOwner resolves the Deployment that owns a ReplicaSet.
func (c *Client) resolveReplicaSetOwner(ctx context.Context, ns, name string) (string, string) {
	rs, err := c.clientset.AppsV1().ReplicaSets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", ""
	}
	for _, ref := range rs.OwnerReferences {
		if ref.Controller != nil && *ref.Controller {
			return ref.Kind, ref.Name
		}
	}
	return "", ""
}
