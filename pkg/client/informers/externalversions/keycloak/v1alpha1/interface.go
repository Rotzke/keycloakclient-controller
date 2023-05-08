/*
Copyright 2022.

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
// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	internalinterfaces "github.com/movewp3/keycloakclient-controller/pkg/client/informers/externalversions/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// Keycloaks returns a KeycloakInformer.
	Keycloaks() KeycloakInformer
	// KeycloakClients returns a KeycloakClientInformer.
	KeycloakClients() KeycloakClientInformer
	// KeycloakRealms returns a KeycloakRealmInformer.
	KeycloakRealms() KeycloakRealmInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

// Keycloaks returns a KeycloakInformer.
func (v *version) Keycloaks() KeycloakInformer {
	return &keycloakInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// KeycloakClients returns a KeycloakClientInformer.
func (v *version) KeycloakClients() KeycloakClientInformer {
	return &keycloakClientInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// KeycloakRealms returns a KeycloakRealmInformer.
func (v *version) KeycloakRealms() KeycloakRealmInformer {
	return &keycloakRealmInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}
