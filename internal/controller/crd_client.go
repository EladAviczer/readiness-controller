package controller

import (
	"context"

	"heartbeat-operator/api/v1alpha1"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

type CrdClient struct {
	restClient rest.Interface
	ns         string
}

func NewCrdClient(config *rest.Config, namespace string) (*CrdClient, error) {
	if err := v1alpha1.AddToScheme(scheme.Scheme); err != nil {
		return nil, err
	}

	crdConfig := *config
	crdConfig.ContentConfig.GroupVersion = &v1alpha1.SchemeGroupVersion
	crdConfig.APIPath = "/apis"
	crdConfig.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	crdConfig.UserAgent = rest.DefaultKubernetesUserAgent()

	client, err := rest.RESTClientFor(&crdConfig)
	if err != nil {
		return nil, err
	}

	return &CrdClient{
		restClient: client,
		ns:         namespace,
	}, nil
}

func (c *CrdClient) Create(ctx context.Context, check *v1alpha1.Probe) (*v1alpha1.Probe, error) {
	result := &v1alpha1.Probe{}
	err := c.restClient.Post().
		Namespace(c.ns).
		Resource("probes").
		Body(check).
		Do(ctx).
		Into(result)
	return result, err
}

func (c *CrdClient) Get(ctx context.Context, name string) (*v1alpha1.Probe, error) {
	result := &v1alpha1.Probe{}
	err := c.restClient.Get().
		Namespace(c.ns).
		Resource("probes").
		Name(name).
		Do(ctx).
		Into(result)
	return result, err
}

func (c *CrdClient) UpdateStatus(ctx context.Context, check *v1alpha1.Probe) (*v1alpha1.Probe, error) {
	result := &v1alpha1.Probe{}
	err := c.restClient.Put().
		Namespace(c.ns).
		Resource("probes").
		Name(check.Name).
		SubResource("status").
		Body(check).
		Do(ctx).
		Into(result)
	return result, err
}
