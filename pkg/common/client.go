package common

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/movewp3/keycloakclient-controller/api/v1alpha1"
	"github.com/movewp3/keycloakclient-controller/pkg/model"
	"github.com/pkg/errors"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	config2 "sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	authURL = "auth/realms/master/protocol/openid-connect/token"
)

var logClient = logf.Log.WithName("common.client")

type Requester interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	requester Requester
	URL       string
	token     string
}

// T is a generic type for keycloak spec resources
type T interface{}

// Generic create function for creating new Keycloak resources
func (c *Client) create(obj T, resourcePath, resourceName string) (string, error) {
	jsonValue, err := json.Marshal(obj)
	if err != nil {
		log.Error(err, "error marshalling object", err)
		return "", nil
	}

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/auth/admin/%s", c.URL, resourcePath),
		bytes.NewBuffer(jsonValue),
	)
	if err != nil {
		log.Error(err, "error creating POST request ", resourceName)
		return "", errors.Wrapf(err, "error creating POST %s request", resourceName)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.token))
	res, err := c.requester.Do(req)

	if err != nil {
		log.Error(err, "error on request ")
		return "", errors.Wrapf(err, "error performing POST %s request", resourceName)
	}
	defer res.Body.Close()

	if res.StatusCode != 201 && res.StatusCode != 204 {
		return "", errors.Errorf("failed to create %s: (%d) %s", resourceName, res.StatusCode, res.Status)
	}

	if resourceName == "client" {
		d, _ := ioutil.ReadAll(res.Body)
		fmt.Println("user response ", string(d))
	}

	location := strings.Split(res.Header.Get("Location"), "/")
	uid := location[len(location)-1]
	return uid, nil
}

func (c *Client) Endpoint() string {
	return c.URL
}

func (c *Client) CreateRealm(realm *v1alpha1.KeycloakRealm) (string, error) {
	return c.create(realm.Spec.Realm, "realms", "realm")
}

func (c *Client) CreateClient(client *v1alpha1.KeycloakAPIClient, realmName string) (string, error) {
	return c.create(client, fmt.Sprintf("realms/%s/clients", realmName), "client")
}

func (c *Client) CreateClientRole(clientID string, role *v1alpha1.RoleRepresentation, realmName string) (string, error) {
	return c.create(role, fmt.Sprintf("realms/%s/clients/%s/roles", realmName, clientID), "client role")
}

func (c *Client) AddRealmRoleComposites(realmName, roleID string, roles *[]v1alpha1.RoleRepresentation) error {
	_, err := c.create(roles, fmt.Sprintf("realms/%s/roles-by-id/%s/composites", realmName, roleID), "realm role composites")
	return err
}

func (c *Client) CreateClientRealmScopeMappings(specClient *v1alpha1.KeycloakAPIClient, mappings *[]v1alpha1.RoleRepresentation, realmName string) error {
	_, err := c.create(mappings, fmt.Sprintf("realms/%s/clients/%s/scope-mappings/realm", realmName, specClient.ID), "client realm scope mappings")
	return err
}

func (c *Client) CreateClientClientScopeMappings(specClient *v1alpha1.KeycloakAPIClient, mappings *v1alpha1.ClientMappingsRepresentation, realmName string) error {
	_, err := c.create(mappings.Mappings, fmt.Sprintf("realms/%s/clients/%s/scope-mappings/clients/%s", realmName, specClient.ID, mappings.ID), "client client scope mappings")
	return err
}

func (c *Client) CreateFederatedIdentity(fid v1alpha1.FederatedIdentity, userID string, realmName string) (string, error) {
	return c.create(fid, fmt.Sprintf("realms/%s/users/%s/federated-identity/%s", realmName, userID, fid.IdentityProvider), "federated-identity")
}

func (c *Client) RemoveFederatedIdentity(fid v1alpha1.FederatedIdentity, userID string, realmName string) error {
	return c.delete(fmt.Sprintf("realms/%s/users/%s/federated-identity/%s", realmName, userID, fid.IdentityProvider), "federated-identity", fid)
}

func (c *Client) GetUserFederatedIdentities(userID string, realmName string) ([]v1alpha1.FederatedIdentity, error) {
	result, err := c.get(fmt.Sprintf("realms/%s/users/%s/federated-identity", realmName, userID), "federated-identity", func(body []byte) (T, error) {
		var fids []v1alpha1.FederatedIdentity
		err := json.Unmarshal(body, &fids)
		return fids, err
	})
	if err != nil {
		return nil, err
	}
	return result.([]v1alpha1.FederatedIdentity), err
}

func (c *Client) CreateUserClientRole(role *v1alpha1.KeycloakUserRole, realmName, clientID, userID string) (string, error) {
	return c.create(
		[]*v1alpha1.KeycloakUserRole{role},
		fmt.Sprintf("realms/%s/users/%s/role-mappings/clients/%s", realmName, userID, clientID),
		"user-client-role",
	)
}
func (c *Client) CreateUserRealmRole(role *v1alpha1.KeycloakUserRole, realmName, userID string) (string, error) {
	return c.create(
		[]*v1alpha1.KeycloakUserRole{role},
		fmt.Sprintf("realms/%s/users/%s/role-mappings/realm", realmName, userID),
		"user-realm-role",
	)
}

func (c *Client) DeleteUserClientRole(role *v1alpha1.KeycloakUserRole, realmName, clientID, userID string) error {
	err := c.delete(
		fmt.Sprintf("realms/%s/users/%s/role-mappings/clients/%s", realmName, userID, clientID),
		"user-client-role",
		[]*v1alpha1.KeycloakUserRole{role},
	)
	return err
}

func (c *Client) DeleteUserRealmRole(role *v1alpha1.KeycloakUserRole, realmName, userID string) error {
	err := c.delete(
		fmt.Sprintf("realms/%s/users/%s/role-mappings/realm", realmName, userID),
		"user-realm-role",
		[]*v1alpha1.KeycloakUserRole{role},
	)
	return err
}

// Generic get function for returning a Keycloak resource
func (c *Client) get(resourcePath, resourceName string, unMarshalFunc func(body []byte) (T, error)) (T, error) {
	u := fmt.Sprintf("%s/auth/admin/%s", c.URL, resourcePath)
	req, err := http.NewRequest(
		"GET",
		u,
		nil,
	)
	if err != nil {
		log.Error(err, "error creating GET request ", resourceName, ": ", err)
		return nil, errors.Wrapf(err, "error creating GET %s request", resourceName)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.token))
	res, err := c.requester.Do(req)
	if err != nil {

		logClient.Error(err, "error on request")
		return nil, errors.Wrapf(err, "error performing GET %s request", resourceName)
	}

	defer res.Body.Close()
	if res.StatusCode == 404 {
		logClient.Error(nil, "Resource %v/%v doesn't exist", resourcePath, resourceName)
		return nil, nil
	}

	if res.StatusCode != 200 {
		return nil, errors.Errorf("failed to GET %s: (%d) %s", resourceName, res.StatusCode, res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logClient.Error(nil, "error reading response %+v", err)
		return nil, errors.Wrapf(err, "error reading %s GET response", resourceName)
	}

	obj, err := unMarshalFunc(body)
	if err != nil {
		logClient.Error(err, "error unmarshalling")
		return nil, err
	}
	return obj, nil
}

func (c *Client) GetRealm(realmName string) (*v1alpha1.KeycloakRealm, error) {
	result, err := c.get(fmt.Sprintf("realms/%s", realmName), "realm", func(body []byte) (T, error) {
		realm := &v1alpha1.KeycloakAPIRealm{}
		err := json.Unmarshal(body, realm)
		return realm, err
	})
	if result == nil {
		return nil, nil
	}
	ret := &v1alpha1.KeycloakRealm{
		Spec: v1alpha1.KeycloakRealmSpec{
			Realm: result.(*v1alpha1.KeycloakAPIRealm),
		},
	}
	return ret, err
}

func (c *Client) GetClient(clientID, realmName string) (*v1alpha1.KeycloakAPIClient, error) {
	result, err := c.get(fmt.Sprintf("realms/%s/clients/%s", realmName, clientID), "client", func(body []byte) (T, error) {
		client := &v1alpha1.KeycloakAPIClient{}
		err := json.Unmarshal(body, client)
		return client, err
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	ret := result.(*v1alpha1.KeycloakAPIClient)
	return ret, err
}

func (c *Client) GetClientID(name, realmName string) (string, error) {
	result, err := c.get(fmt.Sprintf("realms/%s/clients/?clientId=%s", realmName, name), "client", func(body []byte) (T, error) {
		clients := []*v1alpha1.KeycloakAPIClient{}
		err := json.Unmarshal(body, &clients)
		return clients[0].ID, err
	})
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}
	ret := result.(string)
	return ret, err
}

func (c *Client) GetClientSecret(clientID, realmName string) (string, error) {
	//"https://{{ rhsso_route }}/auth/admin/realms/{{ rhsso_realm }}/clients/{{ rhsso_client_id }}/client-secret"
	result, err := c.get(fmt.Sprintf("realms/%s/clients/%s/client-secret", realmName, clientID), "client-secret", func(body []byte) (T, error) {
		res := map[string]string{}
		if err := json.Unmarshal(body, &res); err != nil {
			return nil, err
		}
		return res["value"], nil
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to get: "+fmt.Sprintf("realms/%s/clients/%s/client-secret", realmName, clientID))
	}
	if result == nil {
		return "", nil
	}
	return result.(string), nil
}

func (c *Client) GetClientInstall(clientID, realmName string) ([]byte, error) {
	var response []byte
	if _, err := c.get(fmt.Sprintf("realms/%s/clients/%s/installation/providers/keycloak-oidc-keycloak-json", realmName, clientID), "client-installation", func(body []byte) (T, error) {
		response = body
		return body, nil
	}); err != nil {
		return nil, err
	}
	return response, nil
}

// Generic put function for updating Keycloak resources
func (c *Client) update(obj T, resourcePath, resourceName string) error {
	jsonValue, err := json.Marshal(obj)
	if err != nil {
		return nil
	}

	req, err := http.NewRequest(
		"PUT",
		fmt.Sprintf("%s/auth/admin/%s", c.URL, resourcePath),
		bytes.NewBuffer(jsonValue),
	)
	if err != nil {
		logClient.Error(err, "error creating UPDATE %s request", resourceName)
		return errors.Wrapf(err, "error creating UPDATE %s request", resourceName)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer "+c.token)
	res, err := c.requester.Do(req)
	if err != nil {
		logClient.Error(err, "error on request")
		return errors.Wrapf(err, "error performing UPDATE %s request", resourceName)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		logClient.Error(err, "failed to UPDATE %s %v", resourceName, res.Status)
		return errors.Errorf("failed to UPDATE %s: (%d) %s", resourceName, res.StatusCode, res.Status)
	}

	return nil
}

func (c *Client) UpdateRealm(realm *v1alpha1.KeycloakRealm) error {
	return c.update(realm, fmt.Sprintf("realms/%s", realm.Spec.Realm.ID), "realm")
}

func (c *Client) UpdateClient(specClient *v1alpha1.KeycloakAPIClient, realmName string) error {
	return c.update(specClient, fmt.Sprintf("realms/%s/clients/%s", realmName, specClient.ID), "client")
}

func (c *Client) UpdateClientRole(clientID string, role, oldRole *v1alpha1.RoleRepresentation, realmName string) error {
	return c.update(role, fmt.Sprintf("realms/%s/clients/%s/roles/%s", realmName, clientID, oldRole.Name), "client role")
}

func (c *Client) UpdateClientDefaultClientScope(specClient *v1alpha1.KeycloakAPIClient, clientScope *v1alpha1.KeycloakClientScope, realmName string) error {
	return c.update(clientScope, fmt.Sprintf("realms/%s/clients/%s/default-client-scopes/%s", realmName, specClient.ID, clientScope.ID), "client default client scope")
}

func (c *Client) UpdateClientOptionalClientScope(specClient *v1alpha1.KeycloakAPIClient, clientScope *v1alpha1.KeycloakClientScope, realmName string) error {
	return c.update(clientScope, fmt.Sprintf("realms/%s/clients/%s/optional-client-scopes/%s", realmName, specClient.ID, clientScope.ID), "client optional client scope")
}

// Generic delete function for deleting Keycloak resources
func (c *Client) delete(resourcePath, resourceName string, obj T) error {
	req, err := http.NewRequest(
		"DELETE",
		fmt.Sprintf("%s/auth/admin/%s", c.URL, resourcePath),
		nil,
	)

	if obj != nil {
		jsonValue, err := json.Marshal(obj)
		if err != nil {
			return nil
		}
		req, err = http.NewRequest(
			"DELETE",
			fmt.Sprintf("%s/auth/admin/%s", c.URL, resourcePath),
			bytes.NewBuffer(jsonValue),
		)
		if err != nil {
			return nil
		}
		req.Header.Set("Content-Type", "application/json")
	}

	if err != nil {
		logClient.Error(err, "error creating DELETE %s request", resourceName)
		return errors.Wrapf(err, "error creating DELETE %s request", resourceName)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.token))
	res, err := c.requester.Do(req)
	if err != nil {
		logClient.Error(err, "error on request")
		return errors.Wrapf(err, "error performing DELETE %s request", resourceName)
	}
	defer res.Body.Close()
	if res.StatusCode == 404 {
		logClient.Error(err, "Resource %v/%v already deleted", resourcePath, resourceName)
	}
	if res.StatusCode != 204 && res.StatusCode != 404 {
		return errors.Errorf("failed to DELETE %s: (%d) %s", resourceName, res.StatusCode, res.Status)
	}

	return nil
}

func (c *Client) DeleteRealm(realmName string) error {
	err := c.delete(fmt.Sprintf("realms/%s", realmName), "realm", nil)
	return err
}

func (c *Client) DeleteClient(clientID, realmName string) error {
	err := c.delete(fmt.Sprintf("realms/%s/clients/%s", realmName, clientID), "client", nil)
	return err
}

func (c *Client) DeleteClientRole(clientID, role, realmName string) error {
	err := c.delete(fmt.Sprintf("realms/%s/clients/%s/roles/%s", realmName, clientID, role), "client role", nil)
	return err
}

func (c *Client) DeleteRealmRoleComposites(realmName, roleID string, roles *[]v1alpha1.RoleRepresentation) error {
	return c.delete(fmt.Sprintf("realms/%s/roles-by-id/%s/composites", realmName, roleID), "realm role composites", roles)
}

func (c *Client) DeleteClientRealmScopeMappings(specClient *v1alpha1.KeycloakAPIClient, mappings *[]v1alpha1.RoleRepresentation, realmName string) error {
	return c.delete(fmt.Sprintf("realms/%s/clients/%s/scope-mappings/realm", realmName, specClient.ID), "client realm scope mappings", mappings)
}

func (c *Client) DeleteClientClientScopeMappings(specClient *v1alpha1.KeycloakAPIClient, mappings *v1alpha1.ClientMappingsRepresentation, realmName string) error {
	return c.delete(fmt.Sprintf("realms/%s/clients/%s/scope-mappings/clients/%s", realmName, specClient.ID, mappings.ID), "client client scope mappings", mappings.Mappings)
}

func (c *Client) DeleteClientDefaultClientScope(specClient *v1alpha1.KeycloakAPIClient, clientScope *v1alpha1.KeycloakClientScope, realmName string) error {
	return c.delete(fmt.Sprintf("realms/%s/clients/%s/default-client-scopes/%s", realmName, specClient.ID, clientScope.ID), "client default client scope", clientScope)
}

func (c *Client) DeleteClientOptionalClientScope(specClient *v1alpha1.KeycloakAPIClient, clientScope *v1alpha1.KeycloakClientScope, realmName string) error {
	return c.delete(fmt.Sprintf("realms/%s/clients/%s/optional-client-scopes/%s", realmName, specClient.ID, clientScope.ID), "client optional client scope", clientScope)
}

// Generic list function for listing Keycloak resources
func (c *Client) list(resourcePath, resourceName string, unMarshalListFunc func(body []byte) (T, error)) (T, error) {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/auth/admin/%s", c.URL, resourcePath),
		nil,
	)
	if err != nil {
		logClient.Error(err, "error creating LIST %s request %+v", resourceName, err)
		return nil, errors.Wrapf(err, "error creating LIST %s request", resourceName)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.token))
	res, err := c.requester.Do(req)
	if err != nil {
		logClient.Error(err, "error on request")
		return nil, errors.Wrapf(err, "error performing LIST %s request", resourceName)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, errors.Errorf("failed to LIST %s: (%d) %s", resourceName, res.StatusCode, res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logClient.Error(err, "error reading response")
		return nil, errors.Wrapf(err, "error reading %s LIST response", resourceName)
	}

	objs, err := unMarshalListFunc(body)
	if err != nil {
		logClient.Error(err, "error unmarshalling body")
	}

	return objs, nil
}

func (c *Client) ListRealms() ([]*v1alpha1.KeycloakRealm, error) {
	result, err := c.list("realms", "realm", func(body []byte) (T, error) {
		var realms []*v1alpha1.KeycloakRealm
		err := json.Unmarshal(body, &realms)
		return realms, err
	})
	resultAsRealm, ok := result.([]*v1alpha1.KeycloakRealm)
	if !ok {
		return nil, err
	}
	return resultAsRealm, err
}

func (c *Client) ListRealmRoleClientRoleComposites(realmName, roleID, clientID string) ([]v1alpha1.RoleRepresentation, error) {
	result, err := c.list(fmt.Sprintf("realms/%s/roles-by-id/%s/composites/clients/%s", realmName, roleID, clientID), "realm role client role composites", func(body []byte) (T, error) {
		var roles []v1alpha1.RoleRepresentation
		err := json.Unmarshal(body, &roles)
		return roles, err
	})

	if err != nil {
		return nil, err
	}

	res, ok := result.([]v1alpha1.RoleRepresentation)

	if !ok {
		return nil, errors.Errorf("error decoding list realm role client role composites")
	}

	return res, nil
}

func (c *Client) ListClients(realmName string) ([]*v1alpha1.KeycloakAPIClient, error) {
	result, err := c.list(fmt.Sprintf("realms/%s/clients", realmName), "clients", func(body []byte) (T, error) {
		var clients []*v1alpha1.KeycloakAPIClient
		err := json.Unmarshal(body, &clients)
		return clients, err
	})

	if err != nil {
		return nil, err
	}

	res, ok := result.([]*v1alpha1.KeycloakAPIClient)

	if !ok {
		return nil, errors.Errorf("error decoding list clients response")
	}

	return res, nil
}

func (c *Client) ListClientRoles(clientID, realmName string) ([]v1alpha1.RoleRepresentation, error) {
	result, err := c.list(fmt.Sprintf("realms/%s/clients/%s/roles", realmName, clientID), "client roles", func(body []byte) (T, error) {
		var roles []v1alpha1.RoleRepresentation
		err := json.Unmarshal(body, &roles)
		return roles, err
	})

	if err != nil {
		return nil, err
	}

	res, ok := result.([]v1alpha1.RoleRepresentation)

	if !ok {
		return nil, errors.Errorf("error decoding list client roles response")
	}

	return res, nil
}

func (c *Client) ListScopeMappings(clientID, realmName string) (*v1alpha1.MappingsRepresentation, error) {
	result, err := c.list(fmt.Sprintf("realms/%s/clients/%s/scope-mappings", realmName, clientID), "client scope mappings", func(body []byte) (T, error) {
		var mappings v1alpha1.MappingsRepresentation
		err := json.Unmarshal(body, &mappings)
		return mappings, err
	})

	if err != nil {
		return nil, err
	}

	res, ok := result.(v1alpha1.MappingsRepresentation)

	if !ok {
		return nil, errors.Errorf("error decoding list client scope mappings response")
	}

	return &res, nil
}

func (c *Client) listClientScopes(path string, msg string) ([]v1alpha1.KeycloakClientScope, error) {
	result, err := c.list(path, msg, func(body []byte) (T, error) {
		var assignedClientScopes []v1alpha1.KeycloakClientScope
		err := json.Unmarshal(body, &assignedClientScopes)
		return assignedClientScopes, err
	})

	if err != nil {
		return nil, err
	}

	res, ok := result.([]v1alpha1.KeycloakClientScope)

	if !ok {
		return nil, errors.Errorf("error decoding list %s response", msg)
	}

	return res, nil
}

func (c *Client) ListAvailableClientScopes(realmName string) ([]v1alpha1.KeycloakClientScope, error) {
	return c.listClientScopes(fmt.Sprintf("realms/%s/client-scopes", realmName), "available client scopes")
}

func (c *Client) ListDefaultClientScopes(clientID, realmName string) ([]v1alpha1.KeycloakClientScope, error) {
	return c.listClientScopes(fmt.Sprintf("realms/%s/clients/%s/default-client-scopes", realmName, clientID), "default client scopes")
}

func (c *Client) ListOptionalClientScopes(clientID, realmName string) ([]v1alpha1.KeycloakClientScope, error) {
	return c.listClientScopes(fmt.Sprintf("realms/%s/clients/%s/optional-client-scopes", realmName, clientID), "optional client scopes")
}

func (c *Client) ListUserClientRoles(realmName, clientID, userID string) ([]*v1alpha1.KeycloakUserRole, error) {
	objects, err := c.list("realms/"+realmName+"/users/"+userID+"/role-mappings/clients/"+clientID, "userClientRoles", func(body []byte) (t T, e error) {
		var userClientRoles []*v1alpha1.KeycloakUserRole
		err := json.Unmarshal(body, &userClientRoles)
		return userClientRoles, err
	})
	if err != nil {
		return nil, err
	}
	if objects == nil {
		return nil, nil
	}
	return objects.([]*v1alpha1.KeycloakUserRole), err
}

func (c *Client) ListAvailableUserClientRoles(realmName, clientID, userID string) ([]*v1alpha1.KeycloakUserRole, error) {
	objects, err := c.list("realms/"+realmName+"/users/"+userID+"/role-mappings/clients/"+clientID+"/available", "userClientRoles", func(body []byte) (t T, e error) {
		var userClientRoles []*v1alpha1.KeycloakUserRole
		err := json.Unmarshal(body, &userClientRoles)
		return userClientRoles, err
	})
	if err != nil {
		return nil, err
	}
	if objects == nil {
		return nil, nil
	}
	return objects.([]*v1alpha1.KeycloakUserRole), err
}

func (c *Client) ListUserRealmRoles(realmName, userID string) ([]*v1alpha1.KeycloakUserRole, error) {
	objects, err := c.list("realms/"+realmName+"/users/"+userID+"/role-mappings/realm", "userRealmRoles", func(body []byte) (t T, e error) {
		var userRealmRoles []*v1alpha1.KeycloakUserRole
		err := json.Unmarshal(body, &userRealmRoles)
		return userRealmRoles, err
	})
	if err != nil {
		return nil, err
	}
	if objects == nil {
		return nil, nil
	}
	return objects.([]*v1alpha1.KeycloakUserRole), err
}

func (c *Client) ListAvailableUserRealmRoles(realmName, userID string) ([]*v1alpha1.KeycloakUserRole, error) {
	objects, err := c.list("realms/"+realmName+"/users/"+userID+"/role-mappings/realm/available", "userClientRoles", func(body []byte) (t T, e error) {
		var userRealmRoles []*v1alpha1.KeycloakUserRole
		err := json.Unmarshal(body, &userRealmRoles)
		return userRealmRoles, err
	})
	if err != nil {
		return nil, err
	}
	if objects == nil {
		return nil, nil
	}
	return objects.([]*v1alpha1.KeycloakUserRole), err
}

func (c *Client) Ping() error {
	u := c.URL + "/auth/"
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		logClient.Error(err, "error creating ping request")
		return errors.Wrap(err, "error creating ping request")
	}

	res, err := c.requester.Do(req)
	if err != nil {
		logClient.Error(err, "error on request")
		return errors.Wrapf(err, "error performing ping request")
	}

	logClient.Info("response status: " + strconv.Itoa(res.StatusCode) + " " + res.Status)
	if res.StatusCode != 200 {
		return errors.Errorf("failed to ping, response status code: %v", res.StatusCode)
	}
	defer res.Body.Close()

	return nil
}

func (c *Client) GetServiceAccountUser(realmName, clientID string) (*v1alpha1.KeycloakAPIUser, error) {
	result, err := c.get(fmt.Sprintf("realms/%s/clients/%s/service-account-user", realmName, clientID), "service-account-user", func(body []byte) (T, error) {
		user := &v1alpha1.KeycloakAPIUser{}
		err := json.Unmarshal(body, user)
		return user, err
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	ret := result.(*v1alpha1.KeycloakAPIUser)
	return ret, err
}

// login requests a new auth token from Keycloak
func (c *Client) login_old(user, pass string) error {
	form := url.Values{}
	form.Add("username", user)
	form.Add("password", pass)
	form.Add("client_id", "admin-cli")
	form.Add("grant_type", "password")

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/%s", c.URL, authURL),
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return errors.Wrap(err, "error creating login request")
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.requester.Do(req)
	if err != nil {
		logClient.Error(err, "error on request ")
		return errors.Wrap(err, "error performing token request")
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logClient.Error(err, "error reading response")
		return errors.Wrap(err, "error reading token response")
	}

	tokenRes := &v1alpha1.TokenResponse{}
	err = json.Unmarshal(body, tokenRes)
	if err != nil {
		return errors.Wrap(err, "error parsing token response")
	}

	if tokenRes.Error != "" {
		logClient.Error(errors.New(tokenRes.Error), "error with request: %s", tokenRes.ErrorDescription)
		return errors.Errorf(tokenRes.ErrorDescription)
	}

	c.token = tokenRes.AccessToken

	return nil
}

// login requests a new auth token from Keycloak
func (c *Client) login(client, credential string) error {
	form := url.Values{}

	form.Add("client_id", client)
	form.Add("client_secret", credential)
	form.Add("grant_type", "client_credentials")

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/%s", c.URL, authURL),
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return errors.Wrap(err, "error creating login request")
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.requester.Do(req)
	if err != nil {
		logClient.Error(err, "error on request ")
		return errors.Wrap(err, "error performing token request")
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logClient.Error(err, "error reading response")
		return errors.Wrap(err, "error reading token response")
	}

	tokenRes := &v1alpha1.TokenResponse{}
	err = json.Unmarshal(body, tokenRes)
	if err != nil {
		return errors.Wrap(err, "error parsing token response")
	}

	if tokenRes.Error != "" {
		logClient.Error(errors.New(tokenRes.Error), "error with request: %s", tokenRes.ErrorDescription)
		return errors.Errorf(tokenRes.ErrorDescription)
	}

	c.token = tokenRes.AccessToken
	logClient.Info("login with serviceaccount " + client + " succeeded")
	return nil
}

// defaultRequester returns a default client for requesting http endpoints
func defaultRequester(serverCert []byte) (Requester, error) {
	tlsConfig, err := createTLSConfig(serverCert)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig

	c := &http.Client{Transport: transport, Timeout: time.Second * 10}
	return c, nil
}

// createTLSConfig constructs and returns a TLS Config with a root CA read
// from the serverCert param if present, or a permissive config which
// is insecure otherwise
func createTLSConfig(serverCert []byte) (*tls.Config, error) {
	if serverCert == nil {
		return &tls.Config{InsecureSkipVerify: true}, nil // nolint
	}

	rootCAPool := x509.NewCertPool()
	if ok := rootCAPool.AppendCertsFromPEM(serverCert); !ok {
		return nil, errors.Errorf("unable to successfully load certificate")
	}
	return &tls.Config{RootCAs: rootCAPool}, nil
}

//go:generate moq -out keycloakClient_moq.go . KeycloakInterface

type KeycloakInterface interface {
	Ping() error

	Endpoint() string

	CreateRealm(realm *v1alpha1.KeycloakRealm) (string, error)
	GetRealm(realmName string) (*v1alpha1.KeycloakRealm, error)
	UpdateRealm(specRealm *v1alpha1.KeycloakRealm) error
	DeleteRealm(realmName string) error
	ListRealms() ([]*v1alpha1.KeycloakRealm, error)

	ListRealmRoleClientRoleComposites(realmName, roleID, clientID string) ([]v1alpha1.RoleRepresentation, error)
	AddRealmRoleComposites(realmName, roleID string, roles *[]v1alpha1.RoleRepresentation) error
	DeleteRealmRoleComposites(realmName, roleID string, roles *[]v1alpha1.RoleRepresentation) error

	CreateClient(client *v1alpha1.KeycloakAPIClient, realmName string) (string, error)
	GetClient(clientID, realmName string) (*v1alpha1.KeycloakAPIClient, error)
	GetClientID(clientID, realmName string) (string, error)
	GetClientSecret(clientID, realmName string) (string, error)
	GetClientInstall(clientID, realmName string) ([]byte, error)
	UpdateClient(specClient *v1alpha1.KeycloakAPIClient, realmName string) error
	DeleteClient(clientID, realmName string) error
	ListClients(realmName string) ([]*v1alpha1.KeycloakAPIClient, error)
	ListClientRoles(clientID, realmName string) ([]v1alpha1.RoleRepresentation, error)
	ListScopeMappings(clientID, realmName string) (*v1alpha1.MappingsRepresentation, error)
	ListAvailableClientScopes(realmName string) ([]v1alpha1.KeycloakClientScope, error)
	ListDefaultClientScopes(clientID, realmName string) ([]v1alpha1.KeycloakClientScope, error)
	ListOptionalClientScopes(clientID, realmName string) ([]v1alpha1.KeycloakClientScope, error)
	CreateClientRole(clientID string, role *v1alpha1.RoleRepresentation, realmName string) (string, error)
	UpdateClientRole(clientID string, role, oldRole *v1alpha1.RoleRepresentation, realmName string) error
	DeleteClientRole(clientID, role, realmName string) error
	CreateClientRealmScopeMappings(specClient *v1alpha1.KeycloakAPIClient, mappings *[]v1alpha1.RoleRepresentation, realmName string) error
	DeleteClientRealmScopeMappings(specClient *v1alpha1.KeycloakAPIClient, mappings *[]v1alpha1.RoleRepresentation, realmName string) error
	CreateClientClientScopeMappings(specClient *v1alpha1.KeycloakAPIClient, mappings *v1alpha1.ClientMappingsRepresentation, realmName string) error
	DeleteClientClientScopeMappings(specClient *v1alpha1.KeycloakAPIClient, mappings *v1alpha1.ClientMappingsRepresentation, realmName string) error
	UpdateClientDefaultClientScope(specClient *v1alpha1.KeycloakAPIClient, clientScope *v1alpha1.KeycloakClientScope, realmName string) error
	DeleteClientDefaultClientScope(specClient *v1alpha1.KeycloakAPIClient, clientScope *v1alpha1.KeycloakClientScope, realmName string) error
	UpdateClientOptionalClientScope(specClient *v1alpha1.KeycloakAPIClient, clientScope *v1alpha1.KeycloakClientScope, realmName string) error
	DeleteClientOptionalClientScope(specClient *v1alpha1.KeycloakAPIClient, clientScope *v1alpha1.KeycloakClientScope, realmName string) error

	CreateFederatedIdentity(fid v1alpha1.FederatedIdentity, userID string, realmName string) (string, error)
	RemoveFederatedIdentity(fid v1alpha1.FederatedIdentity, userID string, realmName string) error
	GetUserFederatedIdentities(userName string, realmName string) ([]v1alpha1.FederatedIdentity, error)

	CreateUserClientRole(role *v1alpha1.KeycloakUserRole, realmName, clientID, userID string) (string, error)
	ListUserClientRoles(realmName, clientID, userID string) ([]*v1alpha1.KeycloakUserRole, error)
	ListAvailableUserClientRoles(realmName, clientID, userID string) ([]*v1alpha1.KeycloakUserRole, error)
	DeleteUserClientRole(role *v1alpha1.KeycloakUserRole, realmName, clientID, userID string) error

	CreateUserRealmRole(role *v1alpha1.KeycloakUserRole, realmName, userID string) (string, error)
	ListUserRealmRoles(realmName, userID string) ([]*v1alpha1.KeycloakUserRole, error)
	ListAvailableUserRealmRoles(realmName, userID string) ([]*v1alpha1.KeycloakUserRole, error)
	DeleteUserRealmRole(role *v1alpha1.KeycloakUserRole, realmName, userID string) error

	GetServiceAccountUser(realmName, clientID string) (*v1alpha1.KeycloakAPIUser, error)
}

// check if Client implements KeycloakInterface
var _ KeycloakInterface = &Client{}

//go:generate moq -out keycloakClientFactory_moq.go . KeycloakClientFactory

// KeycloakClientFactory interface
type KeycloakClientFactory interface {
	AuthenticatedClient(kc v1alpha1.Keycloak) (KeycloakInterface, error)
}

type LocalConfigKeycloakFactory struct {
}

// AuthenticatedClient returns an authenticated client for requesting endpoints from the Keycloak api
func (i *LocalConfigKeycloakFactory) AuthenticatedClient(kc v1alpha1.Keycloak, insecureSsl bool) (KeycloakInterface, error) {
	config, err := config2.GetConfig()
	if err != nil {
		return nil, err
	}

	secretClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	var credentialSecret string
	if kc.Spec.External.Enabled {
		credentialSecret = "credential-" + kc.Name
	} else {
		credentialSecret = kc.Status.CredentialSecret
	}

	adminCreds, err := secretClient.CoreV1().Secrets(kc.Namespace).Get(context.TODO(), credentialSecret, v12.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the admin credentials")
	}
	user := string(adminCreds.Data[model.AdminUsernameProperty])
	pass := string(adminCreds.Data[model.AdminPasswordProperty])
	clientName := string(adminCreds.Data[model.ClientName])
	clientCredential := string(adminCreds.Data[model.ClientPassword])

	var serverCert []byte = nil
	if !insecureSsl {
		serverCert, err = getKCServerCert(secretClient, kc)
		if err != nil {
			return nil, err
		}
	}

	requester, err := defaultRequester(serverCert)
	if err != nil {
		return nil, err
	}

	kcURL, err := getKeycloakURL(kc, requester)
	if err != nil {
		return nil, err
	}

	client := &Client{
		URL:       kcURL,
		requester: requester,
	}
	if clientName != "" && clientCredential != "" {
		client.login(clientName, clientCredential)
		if err := client.login_old(user, pass); err != nil {
			if err := client.login_old(user, pass); err != nil {
				return nil, err
			}
		}
	} else {
		if err := client.login_old(user, pass); err != nil {
			return nil, err
		}
	}
	return client, nil
}

func getKCServerCert(secretClient *kubernetes.Clientset, kc v1alpha1.Keycloak) ([]byte, error) {
	sslCertsSecret, err := secretClient.CoreV1().Secrets(kc.Namespace).Get(context.TODO(), model.ServingCertSecretName, v12.GetOptions{})
	switch {
	case err == nil:
		return sslCertsSecret.Data["tls.crt"], nil
	case k8sErrors.IsNotFound(err):
		return nil, nil
	default:
		return nil, err
	}
}

// At normal conditions, Keycloak should be accessible via the internalURL. However, there are some corner cases (like
// operator running locally during development or services being inaccessible due to network policies) which requires
// use of externalURL.
func getKeycloakURL(kc v1alpha1.Keycloak, requester Requester) (string, error) {
	var kcURL string
	var err error

	if kcURL == "" && kc.Status.ExternalURL != "" {
		kcURL, err = validateKeycloakURL(kc.Status.ExternalURL, requester)
		if err != nil {
			return "", err
		}
	}

	if kcURL == "" {
		return "", errors.Errorf("neither internal nor external url is a valid keycloak url (is keycloak instance running?)")
	}

	log.Info(fmt.Sprintf("found keycloak url: %s", kcURL))

	return kcURL, nil
}

func validateKeycloakURL(url string, requester Requester) (string, error) {
	req, err := http.NewRequest(
		"GET",
		url,
		nil,
	)
	if err != nil {
		return "", err
	}

	res, err := requester.Do(req)
	if err != nil {
		log.Info(fmt.Sprintf("%s is not a valid keycloak url : %s", url, err))
		return "", nil
	}
	_ = res.Body.Close()
	return url, nil
}
