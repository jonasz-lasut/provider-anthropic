/*
Copyright 2026 The provider-anthropic Authors.

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

package skill

import (
	"context"
	"errors"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	xperrors "github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	v1beta1 "github.com/jonasz-lasut/provider-anthropic/apis/managedagents/v1beta1"
	"github.com/jonasz-lasut/provider-anthropic/internal/capabilities"
	"github.com/jonasz-lasut/provider-anthropic/internal/clients"
)

const (
	errNotSkill  = "managed resource is not a Skill"
	errNewClient = "cannot build Anthropic client"
	errNewFS     = "cannot initialise skill filesystem"
	errObserve   = "cannot observe Skill"
	errCreate    = "cannot create Skill"
	errUpdate    = "cannot update Skill (create new SkillVersion)"
	errDelete    = "cannot delete Skill"
)

// Setup adds a controller for Skill to the supplied manager.
// Note: no skipDefaultMetadata parameter — SkillParameters has no Metadata field.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1beta1.SkillKind)

	sfs, err := newSkillFS()
	if err != nil {
		return xperrors.Wrap(err, errNewFS)
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1beta1.Skill{}).
		Complete(managed.NewReconciler(mgr,
			resource.ManagedKind(v1beta1.SkillGroupVersionKind),
			managed.WithExternalConnector(&connector{kube: mgr.GetClient(), fs: sfs}),
			managed.WithLogger(o.Logger.WithValues("controller", name)),
			managed.WithPollInterval(o.PollInterval),
			managed.WithManagementPolicies(),
		))
}

// SetupGated registers the Skill controller to start only once the Skill CRD
// is established.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(err)
		}
	}, v1beta1.SkillGroupVersionKind)
	return nil
}

type connector struct {
	kube client.Client
	fs   *capabilities.FS
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	sk, ok := mg.(*v1beta1.Skill)
	if !ok {
		return nil, xperrors.New(errNotSkill)
	}
	cl, err := clients.NewClient(ctx, c.kube, sk)
	if err != nil {
		return nil, xperrors.Wrap(err, errNewClient)
	}
	return &external{client: cl, kube: c.kube, fs: c.fs}, nil
}

type external struct {
	client *anthropic.Client
	kube   client.Client
	fs     *capabilities.FS
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	sk, ok := mg.(*v1beta1.Skill)
	if !ok {
		return managed.ExternalObservation{}, xperrors.New(errNotSkill)
	}

	skillID := meta.GetExternalName(sk)
	if skillID == "" || skillID == sk.GetName() {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	skillResp, err := e.client.Beta.Skills.Get(ctx, skillID, anthropic.BetaSkillGetParams{})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	sk.FromAnthropicSkillObservation(*skillResp)

	if skillResp.LatestVersion != "" {
		verResp, err := e.client.Beta.Skills.Versions.Get(ctx, skillResp.LatestVersion, anthropic.BetaSkillVersionGetParams{
			SkillID: skillID,
		})
		if err == nil {
			sk.FromAnthropicVersionObservation(*verResp)
		}
		// Non-nil errors (transient 5xx, etc.) are intentionally dropped —
		// version fields are best-effort enrichment; the next Observe will retry.
	}

	sk.SetConditions(xpv1.Available())

	secretData, err := clients.GetSecretData(ctx, e.kube,
		sk.Spec.ForProvider.FilesSecretRef.Name,
		sk.GetNamespace(),
	)
	if err != nil {
		return managed.ExternalObservation{}, xperrors.Wrap(err, errObserve)
	}

	fullHex := capabilities.Hash(secretData)

	// If FilesSha256 is nil the status patch from Create hasn't propagated yet
	// (the external-name annotation watch fires a reconcile before the status
	// subresource update reaches the informer cache). Treat the current Secret
	// as already uploaded so we don't create a spurious extra SkillVersion.
	// The hash is written back to status here and persisted by the reconciler.
	if sk.Status.AtProvider.FilesSha256 == nil {
		sk.Status.AtProvider.FilesSha256 = &fullHex
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isUpToDate(sk, fullHex),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	sk, ok := mg.(*v1beta1.Skill)
	if !ok {
		return managed.ExternalCreation{}, xperrors.New(errNotSkill)
	}

	secretData, err := clients.GetSecretData(ctx, e.kube,
		sk.Spec.ForProvider.FilesSecretRef.Name,
		sk.GetNamespace(),
	)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	fullHex := capabilities.Hash(secretData)
	cacheDir, err := e.fs.StageFiles(sk.GetNamespace(), sk.GetName(), prefixKeys(sk.GetName(), secretData), fullHex[:8])
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	readers, err := e.fs.CollectReaders(cacheDir)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	newParams := sk.ToAnthropicNew()
	newParams.Files = readers
	skillResp, err := e.client.Beta.Skills.New(ctx, newParams)
	if err != nil {
		return managed.ExternalCreation{}, xperrors.Wrap(err, errCreate)
	}

	skillID := skillResp.ID
	meta.SetExternalName(sk, skillID)
	sk.Status.AtProvider.ID = &skillID

	// Skills.New already created the initial version; fetch its details.
	if verResp, err := e.client.Beta.Skills.Versions.Get(ctx, skillResp.LatestVersion, anthropic.BetaSkillVersionGetParams{
		SkillID: skillID,
	}); err == nil {
		sk.FromAnthropicVersionObservation(*verResp)
	}
	sk.Status.AtProvider.FilesSha256 = &fullHex

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	sk, ok := mg.(*v1beta1.Skill)
	if !ok {
		return managed.ExternalUpdate{}, xperrors.New(errNotSkill)
	}

	skillID := meta.GetExternalName(sk)
	if skillID == "" || skillID == sk.GetName() {
		return managed.ExternalUpdate{}, xperrors.New("external name not yet set; skipping update")
	}

	secretData, err := clients.GetSecretData(ctx, e.kube,
		sk.Spec.ForProvider.FilesSecretRef.Name,
		sk.GetNamespace(),
	)
	if err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	fullHex := capabilities.Hash(secretData)
	cacheDir, err := e.fs.StageFiles(sk.GetNamespace(), sk.GetName(), prefixKeys(sk.GetName(), secretData), fullHex[:8])
	if err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	readers, err := e.fs.CollectReaders(cacheDir)
	if err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	verParams := sk.ToAnthropicNewVersion()
	verParams.Files = readers
	verResp, err := e.client.Beta.Skills.Versions.New(ctx, skillID, verParams)
	if err != nil {
		return managed.ExternalUpdate{}, xperrors.Wrap(err, errUpdate)
	}

	populateVersionStatus(sk, verResp)
	sk.Status.AtProvider.FilesSha256 = &fullHex

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	sk, ok := mg.(*v1beta1.Skill)
	if !ok {
		return managed.ExternalDelete{}, xperrors.New(errNotSkill)
	}

	skillID := meta.GetExternalName(sk)
	if skillID == "" || skillID == sk.GetName() {
		return managed.ExternalDelete{}, nil
	}

	// The API rejects Skills.Delete when versions exist; delete them all first.
	pager := e.client.Beta.Skills.Versions.ListAutoPaging(ctx, skillID, anthropic.BetaSkillVersionListParams{})
	for pager.Next() {
		ver := pager.Current()
		_, err := e.client.Beta.Skills.Versions.Delete(ctx, ver.Version, anthropic.BetaSkillVersionDeleteParams{
			SkillID: skillID,
		})
		if err != nil {
			var apiErr *anthropic.Error
			if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
				continue
			}
			return managed.ExternalDelete{}, xperrors.Wrap(err, errDelete)
		}
	}
	if err := pager.Err(); err != nil {
		return managed.ExternalDelete{}, xperrors.Wrap(err, errDelete)
	}

	_, err := e.client.Beta.Skills.Delete(ctx, skillID, anthropic.BetaSkillDeleteParams{})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, xperrors.Wrap(err, errDelete)
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error { return nil }

// populateVersionStatus copies fields from a BetaSkillVersionNewResponse into
// the Skill's AtProvider status. It mirrors FromAnthropicVersionObservation but
// accepts the New-variant response type (which has identical exported fields).
func populateVersionStatus(sk *v1beta1.Skill, resp *anthropic.BetaSkillVersionNewResponse) {
	sk.Status.AtProvider.LatestVersion = &resp.Version
	sk.Status.AtProvider.LatestVersionID = &resp.ID
	sk.Status.AtProvider.LatestVersionName = &resp.Name
	sk.Status.AtProvider.LatestVersionDescription = &resp.Description
	sk.Status.AtProvider.LatestVersionDirectory = &resp.Directory
	sk.Status.AtProvider.LatestVersionCreatedAt = &resp.CreatedAt
}

// prefixKeys returns a copy of data with every key prefixed by "<name>/".
// Secret keys are flat filenames (e.g. "SKILL.md"); the Anthropic API requires
// files to live inside a single top-level directory, so the skill name is used
// as that directory. The hash is always computed on the original flat data.
func prefixKeys(name string, data map[string][]byte) map[string][]byte {
	out := make(map[string][]byte, len(data))
	for k, v := range data {
		out[name+"/"+k] = v
	}
	return out
}

// isUpToDate returns true if the skill's observed file hash matches fullHex.
// There is no JSON subset diff — ForProvider and AtProvider share no comparable
// keys (displayTitle is immutable and its drift is not detected).
func isUpToDate(sk *v1beta1.Skill, fullHex string) bool {
	return sk.Status.AtProvider.FilesSha256 != nil &&
		*sk.Status.AtProvider.FilesSha256 == fullHex
}
