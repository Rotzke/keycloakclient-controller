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
// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/movewp3/keycloakclient-controller/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// KeycloakRealmLister helps list KeycloakRealms.
// All objects returned here must be treated as read-only.
type KeycloakRealmLister interface {
	// List lists all KeycloakRealms in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.KeycloakRealm, err error)
	// KeycloakRealms returns an object that can list and get KeycloakRealms.
	KeycloakRealms(namespace string) KeycloakRealmNamespaceLister
	KeycloakRealmListerExpansion
}

// keycloakRealmLister implements the KeycloakRealmLister interface.
type keycloakRealmLister struct {
	indexer cache.Indexer
}

// NewKeycloakRealmLister returns a new KeycloakRealmLister.
func NewKeycloakRealmLister(indexer cache.Indexer) KeycloakRealmLister {
	return &keycloakRealmLister{indexer: indexer}
}

// List lists all KeycloakRealms in the indexer.
func (s *keycloakRealmLister) List(selector labels.Selector) (ret []*v1alpha1.KeycloakRealm, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.KeycloakRealm))
	})
	return ret, err
}

// KeycloakRealms returns an object that can list and get KeycloakRealms.
func (s *keycloakRealmLister) KeycloakRealms(namespace string) KeycloakRealmNamespaceLister {
	return keycloakRealmNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// KeycloakRealmNamespaceLister helps list and get KeycloakRealms.
// All objects returned here must be treated as read-only.
type KeycloakRealmNamespaceLister interface {
	// List lists all KeycloakRealms in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.KeycloakRealm, err error)
	// Get retrieves the KeycloakRealm from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.KeycloakRealm, error)
	KeycloakRealmNamespaceListerExpansion
}

// keycloakRealmNamespaceLister implements the KeycloakRealmNamespaceLister
// interface.
type keycloakRealmNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all KeycloakRealms in the indexer for a given namespace.
func (s keycloakRealmNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.KeycloakRealm, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.KeycloakRealm))
	})
	return ret, err
}

// Get retrieves the KeycloakRealm from the indexer for a given namespace and name.
func (s keycloakRealmNamespaceLister) Get(name string) (*v1alpha1.KeycloakRealm, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(schema.GroupResource{Group: "keycloak.org", Resource: "keycloakrealm"}, name)
	}
	return obj.(*v1alpha1.KeycloakRealm), nil
}
