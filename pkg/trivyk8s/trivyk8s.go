package trivyk8s

import (
	"context"
	"fmt"

	"github.com/aquasecurity/trivy-kubernetes/pkg/artifacts"
	"github.com/aquasecurity/trivy-kubernetes/pkg/k8s"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	// import auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

// TrivyK8S interface represents the operations supported by the library
type TrivyK8S interface {
	Namespace(string) TrivyK8S
	ArtifactsK8S
}

// ArtifactsK8S interface represents operations to query the artifacts
type ArtifactsK8S interface {
	// ListArtifacts returns kubernetes scanable artifacts
	ListArtifacts(context.Context) ([]*artifacts.Artifact, error)
}

type client struct {
	cluster   k8s.Cluster
	namespace string
}

// New creates a trivyK8S client
func New(cluster k8s.Cluster) TrivyK8S {
	return &client{cluster: cluster}
}

// Namespace configure the namespace to execute the queries
func (c *client) Namespace(namespace string) TrivyK8S {
	c.namespace = namespace
	return c
}

// ListArtifacts returns kubernetes scannable artifacs.
func (c *client) ListArtifacts(ctx context.Context) ([]*artifacts.Artifact, error) {
	artifactList := make([]*artifacts.Artifact, 0)

	namespaced := len(c.namespace) != 0

	grvs, err := c.cluster.GetGVRs(namespaced)
	if err != nil {
		return nil, err
	}

	k8s := c.cluster.GetDynamicClient()
	for _, gvr := range grvs {
		var dclient dynamic.ResourceInterface
		if namespaced {
			dclient = k8s.Resource(gvr).Namespace(c.namespace)
		} else {
			dclient = k8s.Resource(gvr)
		}

		resources, err := dclient.List(ctx, v1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed listing resources for gvr: %v - %w", gvr, err)
		}

		for _, resource := range resources.Items {
			if ignoreResource(resource) {
				continue
			}
			artifact, err := artifacts.FromResource(resource)
			if err != nil {
				return nil, err
			}

			artifactList = append(artifactList, artifact)
		}
	}

	return artifactList, nil
}

// ignore resources to avoid duplication,
// when a resource has an owner, the image/iac will be scanned on the owner itself
func ignoreResource(resource unstructured.Unstructured) bool {
	switch resource.GetKind() {
	case k8s.KindPod, k8s.KindJob, k8s.KindReplicaSet:
		metadata := resource.GetOwnerReferences()
		if metadata != nil {
			return true
		}
	}

	return false
}
