/*
Copyright 2025 The provider-anthropic-platform Authors.

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

// Package observedagentcollection reconciles ObservedAgentCollection
// resources by listing Agents from the Anthropic API and materializing one
// Observe-only Agent per match.
package observedagentcollection

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/pkg/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	xperrors "github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	betav1alpha1 "github.com/jonasz-lasut/provider-anthropic-platform/apis/beta/v1alpha1"
	"github.com/jonasz-lasut/provider-anthropic-platform/internal/clients"
	"github.com/jonasz-lasut/provider-anthropic-platform/internal/predicates"
	"k8s.io/utils/ptr"
)

const (
	errGetCollection    = "cannot get ObservedAgentCollection"
	errNewClient        = "cannot build Anthropic client"
	errListUpstream     = "cannot list Agents from Anthropic API"
	errClientSideFilter = "cannot apply client-side predicate"
	errPatchChild       = "cannot apply observed Agent child"
	errListChildren     = "cannot list child Agents"
	errDeleteChild      = "cannot delete stale observed Agent child"
	errStatusUpdate     = "cannot update ObservedAgentCollection status"
	membershipLabelKey  = "anthropic.crossplane.io/owned-by-collection"
	fieldOwner          = client.FieldOwner("anthropic.crossplane.io/observed-agent-collection-controller")
)

// Reconciler observes Agents from the Anthropic API.
type Reconciler struct {
	client       client.Client
	log          logging.Logger
	pollInterval func() time.Duration
}

// Setup registers the controller with the supplied manager.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := "observedagentcollection"
	r := &Reconciler{
		client:       mgr.GetClient(),
		log:          o.Logger.WithValues("controller", name),
		pollInterval: func() time.Duration { return o.PollInterval },
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		For(&betav1alpha1.ObservedAgentCollection{}).
		WithEventFilter(resource.DesiredStateChanged()).
		Complete(ratelimiter.NewReconciler(name, xperrors.WithSilentRequeueOnConflict(r), o.GlobalRateLimiter))
}

// SetupGated registers Setup behind the CRD-established gate.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			mgr.GetLogger().Error(err, "unable to setup reconciler", "gvk",
				betav1alpha1.ObservedAgentCollectionGroupVersionKind.String())
		}
	}, betav1alpha1.ObservedAgentCollectionGroupVersionKind)
	return nil
}

// Reconcile implements ctrl.Reconciler.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.log.WithValues("request", req)

	c := &betav1alpha1.ObservedAgentCollection{}
	if err := r.client.Get(ctx, req.NamespacedName, c); err != nil {
		if kerrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errors.Wrap(err, errGetCollection)
	}

	if meta.WasDeleted(c) {
		return ctrl.Result{}, nil
	}

	if meta.IsPaused(c) {
		c.Status.SetConditions(xpv1.ReconcilePaused())
		return ctrl.Result{}, errors.Wrap(r.client.Status().Update(ctx, c), errStatusUpdate)
	}

	log.Info("Reconciling")

	cl, err := clients.NewClientFromProviderConfig(ctx, r.client, &c.Spec.ProviderConfigReference, c.GetNamespace())
	if err != nil {
		werr := errors.Wrap(err, errNewClient)
		c.Status.SetConditions(xpv1.ReconcileError(werr))
		_ = r.client.Status().Update(ctx, c)
		return ctrl.Result{}, werr
	}

	listParams := buildListParams(c.Spec.Predicates)
	pager := cl.Beta.Agents.ListAutoPaging(ctx, listParams)
	var matches []anthropic.BetaManagedAgentsAgent
	for pager.Next() {
		matches = append(matches, pager.Current())
	}
	if err := pager.Err(); err != nil {
		werr := errors.Wrap(err, errListUpstream)
		c.Status.SetConditions(xpv1.ReconcileError(werr))
		_ = r.client.Status().Update(ctx, c)
		return ctrl.Result{}, werr
	}

	var filtered []anthropic.BetaManagedAgentsAgent
	for i := range matches {
		ok, ferr := predicates.ClientSideFilter(c.Spec.Predicates, &matches[i])
		if ferr != nil {
			werr := errors.Wrap(ferr, errClientSideFilter)
			c.Status.SetConditions(xpv1.ReconcileError(werr))
			_ = r.client.Status().Update(ctx, c)
			return ctrl.Result{}, werr
		}
		if ok {
			filtered = append(filtered, matches[i])
		}
	}
	matches = filtered

	ml := map[string]string{membershipLabelKey: c.Name}
	keep := sets.New[string]()
	items := make([]betav1alpha1.ObservedCollectionMember, 0, len(matches))
	for i := range matches {
		m := &matches[i]
		childName := childResourceName(c.Name, m.ID)
		child := buildChildPatch(c, childName, m, ml)
		patchData, err := json.Marshal(child)
		if err != nil {
			werr := errors.Wrapf(err, "%s (id=%s)", errPatchChild, m.ID)
			c.Status.SetConditions(xpv1.ReconcileError(werr))
			_ = r.client.Status().Update(ctx, c)
			return ctrl.Result{}, werr
		}
		if err := r.client.Patch(ctx, child, client.RawPatch(types.ApplyPatchType, patchData), fieldOwner, client.ForceOwnership); err != nil {
			werr := errors.Wrapf(err, "%s (id=%s)", errPatchChild, m.ID)
			c.Status.SetConditions(xpv1.ReconcileError(werr))
			_ = r.client.Status().Update(ctx, c)
			return ctrl.Result{}, werr
		}
		keep.Insert(childName)
		items = append(items, betav1alpha1.ObservedCollectionMember{Name: childName, ID: m.ID})
	}

	childList := &betav1alpha1.AgentList{}
	if err := r.client.List(ctx, childList,
		client.MatchingLabels(ml),
		client.InNamespace(c.GetNamespace()),
	); err != nil {
		werr := errors.Wrap(err, errListChildren)
		c.Status.SetConditions(xpv1.ReconcileError(werr))
		_ = r.client.Status().Update(ctx, c)
		return ctrl.Result{}, werr
	}
	for i := range childList.Items {
		if keep.Has(childList.Items[i].Name) {
			continue
		}
		if err := r.client.Delete(ctx, &childList.Items[i]); err != nil && !kerrors.IsNotFound(err) {
			werr := errors.Wrapf(err, "%s name=%s", errDeleteChild, childList.Items[i].Name)
			c.Status.SetConditions(xpv1.ReconcileError(werr))
			_ = r.client.Status().Update(ctx, c)
			return ctrl.Result{}, werr
		}
	}

	count := int32(len(items))
	c.Status.Items = items
	c.Status.ItemCount = &count
	c.Status.MembershipLabel = ml
	c.Status.SetConditions(xpv1.ReconcileSuccess(), xpv1.Available())

	return ctrl.Result{RequeueAfter: r.pollInterval()}, r.client.Status().Update(ctx, c)
}

func childResourceName(collection, id string) string {
	h := sha256.Sum256([]byte(id))
	return fmt.Sprintf("%s-%s", collection, fmt.Sprintf("%x", h)[:7])
}

func buildListParams(p *betav1alpha1.Predicates) anthropic.BetaAgentListParams {
	params := anthropic.BetaAgentListParams{}
	if p == nil {
		return params
	}
	if p.CreatedAtGte != nil {
		params.CreatedAtGte = param.NewOpt(p.CreatedAtGte.Time)
	}
	if p.CreatedAtLte != nil {
		params.CreatedAtLte = param.NewOpt(p.CreatedAtLte.Time)
	}
	return params
}

func buildChildPatch(
	c *betav1alpha1.ObservedAgentCollection,
	childName string,
	item *anthropic.BetaManagedAgentsAgent,
	membership map[string]string,
) *betav1alpha1.Agent {
	labels := map[string]string{}
	for k, v := range membership {
		labels[k] = v
	}
	annotations := map[string]string{
		meta.AnnotationKeyExternalName: item.ID,
	}
	if t := c.Spec.Template; t != nil {
		for k, v := range t.Metadata.Labels {
			labels[k] = v
		}
		for k, v := range t.Metadata.Annotations {
			annotations[k] = v
		}
	}

	child := &betav1alpha1.Agent{
		TypeMeta: metav1.TypeMeta{
			APIVersion: betav1alpha1.GroupVersion.String(),
			Kind:       betav1alpha1.AgentKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        childName,
			Namespace:   c.GetNamespace(),
			Labels:      labels,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         betav1alpha1.GroupVersion.String(),
					Kind:               betav1alpha1.ObservedAgentCollectionKind,
					Name:               c.GetName(),
					UID:                c.GetUID(),
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				},
			},
		},
	}
	pcRef := c.Spec.ProviderConfigReference
	child.Spec.ProviderConfigReference = &pcRef
	child.Spec.ManagementPolicies = xpv1.ManagementPolicies{xpv1.ManagementActionObserve}
	populateForProvider(&child.Spec.ForProvider, item)
	return child
}

// populateForProvider copies observable Agent fields from the API item into
// ForProvider so the child CRD admits successfully. Observe-only policy
// prevents anything written here from being pushed back to the API.
func populateForProvider(p *betav1alpha1.AgentParameters, item *anthropic.BetaManagedAgentsAgent) {
	name := item.Name
	p.Name = &name
	model := item.Model.ID
	p.Model = &model
	if item.Description != "" {
		d := item.Description
		p.Description = &d
	}
	// SystemSecretRef cannot be back-populated from the API response;
	// collection children use an empty ref (no system prompt in spec).
	if len(item.Metadata) > 0 {
		p.Metadata = item.Metadata
	}
}

