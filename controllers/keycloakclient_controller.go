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

package controllers

import (
	"context"
	errors2 "errors"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/reddec/keycloak-ext-operator/internal"
	v12 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/strings/slices"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	keycloakv1alpha1 "github.com/reddec/keycloak-ext-operator/api/v1alpha1"
)

// KeycloakClientReconciler reconciles a KeycloakClient object
type KeycloakClientReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Keycloak *internal.Keycloak
}

const keycloakFinalizer = "reddec.net.k8s.keycloak-finalizer"

//+kubebuilder:rbac:groups=keycloak.k8s.reddec.net,resources=keycloakclients,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=keycloak.k8s.reddec.net,resources=keycloakclients/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=keycloak.k8s.reddec.net,resources=keycloakclients/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *KeycloakClientReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	clientSpec := &keycloakv1alpha1.KeycloakClient{}
	err := r.Get(ctx, req.NamespacedName, clientSpec)
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	if err != nil {
		logger.Error(err, "get client spec")
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if clientSpec.GetDeletionTimestamp() != nil {
		if err := r.removeClient(ctx, clientSpec); err != nil {
			logger.Error(err, "Failed to remove client")
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(clientSpec, keycloakFinalizer)
		if err := r.Update(ctx, clientSpec); err != nil {
			return ctrl.Result{}, err
		}
		log.Log.Info("Client removed")
		return ctrl.Result{}, nil
	}

	// add finalizer (to clean up Keycloak client)
	if !controllerutil.ContainsFinalizer(clientSpec, keycloakFinalizer) {
		controllerutil.AddFinalizer(clientSpec, keycloakFinalizer)
		if err := r.Update(ctx, clientSpec); err != nil {
			return ctrl.Result{}, err
		}
	}

	// get existent keycloak client (by ID or by name as domain) or create new one
	keycloakClient, err := r.getOrCreateClient(ctx, string(clientSpec.UID), clientSpec)
	if err != nil {
		logger.Error(err, "Create client")
		return ctrl.Result{}, err
	}

	// sync manifest and keycloak
	if err := r.updateClient(ctx, keycloakClient, clientSpec.Spec); err != nil {
		logger.Error(err, "Update client")
		return ctrl.Result{}, err
	}

	// Check if the secret already exists, if not create a new one
	secret, err := r.getOrCreateSecret(ctx, keycloakClient, clientSpec)
	if err != nil {
		logger.Error(err, "Failed to get or create Secret")
		return ctrl.Result{}, err
	}

	// Ensure the secret is the same as the spec
	err = r.updateSecret(ctx, secret, clientSpec)
	if err != nil {
		logger.Error(err, "Failed to update Secret", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KeycloakClientReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&keycloakv1alpha1.KeycloakClient{}).
		Owns(&v12.Secret{}).
		Complete(r)
}

func (r *KeycloakClientReconciler) getOrCreateSecret(ctx context.Context, info *internal.ClientDetails, clientSpec *keycloakv1alpha1.KeycloakClient) (*v12.Secret, error) {
	found := &v12.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: clientSpec.SecretName(), Namespace: clientSpec.Namespace}, found)
	if err == nil {
		return found, nil
	}
	if errors.IsNotFound(err) {
		return r.createSecret(ctx, info, clientSpec)
	}
	return nil, err
}

func (r *KeycloakClientReconciler) createSecret(ctx context.Context, info *internal.ClientDetails, manifest *keycloakv1alpha1.KeycloakClient) (*v12.Secret, error) {
	sec := &v12.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      manifest.SecretName(),
			Namespace: manifest.Namespace,
			Labels: map[string]string{
				"keycloak-cr": manifest.Name,
				"keycloak-id": info.ID,
			},
		},
		Immutable: proto.Bool(true),
		Data: map[string][]byte{
			"clientID":     []byte(info.ClientID),
			"clientSecret": []byte(info.Secret),
			"realm":        []byte(manifest.Spec.Realm),
			"realmURL":     []byte(r.Keycloak.RealmURL(manifest.Spec.Realm)),
			"discoveryURL": []byte(r.Keycloak.DiscoveryURL(manifest.Spec.Realm)),
		},
		Type: "Opaque",
	}

	if err := ctrl.SetControllerReference(manifest, sec, r.Scheme); err != nil {
		return nil, fmt.Errorf("set controller refrence: %w", err)
	}
	log.Log.Info("New secret will be created", "Namespace", sec.Namespace, "Name", sec.Name)
	return sec, r.Create(ctx, sec)
}

func (r *KeycloakClientReconciler) updateSecret(ctx context.Context, secret *v12.Secret, m *keycloakv1alpha1.KeycloakClient) error {
	info, err := r.getOrCreateClient(ctx, string(m.UID), m)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	secret.Labels = map[string]string{
		"keycloak-cr": m.Name,
		"keycloak-id": info.ID,
	}
	secret.Data = map[string][]byte{
		"clientID":     []byte(info.ClientID),
		"clientSecret": []byte(info.Secret),
		"realm":        []byte(m.Spec.Realm),
		"realmURL":     []byte(r.Keycloak.RealmURL(m.Spec.Realm)),
		"discoveryURL": []byte(r.Keycloak.DiscoveryURL(m.Spec.Realm)),
	}
	secret.Type = "Opaque"
	return r.Update(ctx, secret)
}

func mostlyTheSame(spec keycloakv1alpha1.KeycloakClientSpec, info *internal.ClientDetails) (internal.ClientDraft, bool) {
	draft := internal.Generate(spec.Domain)
	draft.ClientSecret = info.Secret
	draft.ClientID = info.ClientID
	draft.ID = info.ID
	draft.Description = info.Description
	return draft, draft.Name == info.Name &&
		draft.RootURL == info.RootURL &&
		draft.AdminURL == info.AdminURL &&
		slices.Equal(draft.RedirectURIs, info.RedirectURIs) &&
		slices.Equal(draft.WebOrigins, info.WebOrigins)
}

func (r *KeycloakClientReconciler) updateClient(ctx context.Context, info *internal.ClientDetails, spec keycloakv1alpha1.KeycloakClientSpec) error {
	diff, same := mostlyTheSame(spec, info)
	if same {
		return nil
	}
	// update client urls and name, keep creds the same
	id := diff.ID
	diff.ID = ""
	if err := r.Keycloak.Authorize(ctx).Update(ctx, id, spec.Realm, diff); err != nil {
		return fmt.Errorf("update current client: %w", err)
	}
	log.Log.Info("Keycloak client synced with manifest")
	return nil
}

func (r *KeycloakClientReconciler) getOrCreateClient(ctx context.Context, id string, info *keycloakv1alpha1.KeycloakClient) (*internal.ClientDetails, error) {

	kClient := r.Keycloak.Authorize(ctx)

	existent, err := internal.Find(ctx, kClient, info.Spec.Realm, id, info.Spec.Domain)
	if err == nil {
		return existent, nil
	}
	if !errors2.Is(err, internal.ErrClientNotFound) {
		return nil, fmt.Errorf("get client: %w", err)
	}

	// create new
	draft := internal.Generate(info.Spec.Domain)
	draft.ID = id
	draft.Description = "managed by kubernetes operator"

	_, err = kClient.Create(ctx, info.Spec.Realm, draft)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}
	log.Log.Info("Client created", "client_name", draft.Name)
	return kClient.Get(ctx, info.Spec.Realm, id)
}

func (r *KeycloakClientReconciler) removeClient(ctx context.Context, spec *keycloakv1alpha1.KeycloakClient) error {
	kClient := r.Keycloak.Authorize(ctx)
	info, err := internal.Find(ctx, kClient, spec.Spec.Realm, string(spec.UID), spec.Spec.Domain)
	if err != nil {
		return err
	}
	return kClient.Delete(ctx, spec.Spec.Realm, info.ID)
}
