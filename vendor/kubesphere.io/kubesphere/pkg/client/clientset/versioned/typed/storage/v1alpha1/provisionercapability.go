/*
Copyright 2020 The KubeSphere Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
	v1alpha1 "kubesphere.io/api/storage/v1alpha1"
	scheme "kubesphere.io/kubesphere/pkg/client/clientset/versioned/scheme"
)

// ProvisionerCapabilitiesGetter has a method to return a ProvisionerCapabilityInterface.
// A group's client should implement this interface.
type ProvisionerCapabilitiesGetter interface {
	ProvisionerCapabilities() ProvisionerCapabilityInterface
}

// ProvisionerCapabilityInterface has methods to work with ProvisionerCapability resources.
type ProvisionerCapabilityInterface interface {
	Create(ctx context.Context, provisionerCapability *v1alpha1.ProvisionerCapability, opts v1.CreateOptions) (*v1alpha1.ProvisionerCapability, error)
	Update(ctx context.Context, provisionerCapability *v1alpha1.ProvisionerCapability, opts v1.UpdateOptions) (*v1alpha1.ProvisionerCapability, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.ProvisionerCapability, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.ProvisionerCapabilityList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ProvisionerCapability, err error)
	ProvisionerCapabilityExpansion
}

// provisionerCapabilities implements ProvisionerCapabilityInterface
type provisionerCapabilities struct {
	client rest.Interface
}

// newProvisionerCapabilities returns a ProvisionerCapabilities
func newProvisionerCapabilities(c *StorageV1alpha1Client) *provisionerCapabilities {
	return &provisionerCapabilities{
		client: c.RESTClient(),
	}
}

// Get takes name of the provisionerCapability, and returns the corresponding provisionerCapability object, and an error if there is any.
func (c *provisionerCapabilities) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.ProvisionerCapability, err error) {
	result = &v1alpha1.ProvisionerCapability{}
	err = c.client.Get().
		Resource("provisionercapabilities").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of ProvisionerCapabilities that match those selectors.
func (c *provisionerCapabilities) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.ProvisionerCapabilityList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.ProvisionerCapabilityList{}
	err = c.client.Get().
		Resource("provisionercapabilities").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested provisionerCapabilities.
func (c *provisionerCapabilities) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("provisionercapabilities").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a provisionerCapability and creates it.  Returns the server's representation of the provisionerCapability, and an error, if there is any.
func (c *provisionerCapabilities) Create(ctx context.Context, provisionerCapability *v1alpha1.ProvisionerCapability, opts v1.CreateOptions) (result *v1alpha1.ProvisionerCapability, err error) {
	result = &v1alpha1.ProvisionerCapability{}
	err = c.client.Post().
		Resource("provisionercapabilities").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(provisionerCapability).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a provisionerCapability and updates it. Returns the server's representation of the provisionerCapability, and an error, if there is any.
func (c *provisionerCapabilities) Update(ctx context.Context, provisionerCapability *v1alpha1.ProvisionerCapability, opts v1.UpdateOptions) (result *v1alpha1.ProvisionerCapability, err error) {
	result = &v1alpha1.ProvisionerCapability{}
	err = c.client.Put().
		Resource("provisionercapabilities").
		Name(provisionerCapability.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(provisionerCapability).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the provisionerCapability and deletes it. Returns an error if one occurs.
func (c *provisionerCapabilities) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("provisionercapabilities").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *provisionerCapabilities) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("provisionercapabilities").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched provisionerCapability.
func (c *provisionerCapabilities) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.ProvisionerCapability, err error) {
	result = &v1alpha1.ProvisionerCapability{}
	err = c.client.Patch(pt).
		Resource("provisionercapabilities").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}