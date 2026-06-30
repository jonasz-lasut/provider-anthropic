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

// Package providerconfig implements the ProviderConfig reconciler.
package providerconfig

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/providerconfig"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"

	pcv1beta1 "github.com/jonasz-lasut/provider-anthropic/apis/config/v1beta1"
)

// Setup adds a controller that reconciles ProviderConfigs and
// ClusterProviderConfigs by accounting for their current usage.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	if err := setupNamespacedProviderConfig(mgr, o); err != nil {
		return err
	}
	return setupClusterProviderConfig(mgr, o)

}

func setupNamespacedProviderConfig(mgr ctrl.Manager, o controller.Options) error {
	name := providerconfig.ControllerName(pcv1beta1.ProviderConfigGroupKind)
	of := resource.ProviderConfigKinds{
		Config:    pcv1beta1.ProviderConfigGroupVersionKind,
		Usage:     pcv1beta1.ProviderConfigUsageGroupVersionKind,
		UsageList: pcv1beta1.ProviderConfigUsageListGroupVersionKind,
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&pcv1beta1.ProviderConfig{}).
		Watches(&pcv1beta1.ProviderConfigUsage{}, &resource.EnqueueRequestForProviderConfig{Kind: "ProviderConfig"}).
		Complete(providerconfig.NewReconciler(mgr, of,
			providerconfig.WithLogger(o.Logger.WithValues("controller", name)),
			providerconfig.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))) //nolint:staticcheck // crossplane-runtime does not support new events API yet (crossplane/crossplane#7152)
}

func setupClusterProviderConfig(mgr ctrl.Manager, o controller.Options) error {
	name := providerconfig.ControllerName(pcv1beta1.ClusterProviderConfigGroupKind)
	of := resource.ProviderConfigKinds{
		Config: pcv1beta1.ClusterProviderConfigGroupVersionKind,
		// Usage types are shared
		Usage:     pcv1beta1.ProviderConfigUsageGroupVersionKind,
		UsageList: pcv1beta1.ProviderConfigUsageListGroupVersionKind,
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&pcv1beta1.ClusterProviderConfig{}).
		// Usage types are shared
		Watches(&pcv1beta1.ProviderConfigUsage{}, &resource.EnqueueRequestForProviderConfig{Kind: "ClusterProviderConfig"}).
		Complete(providerconfig.NewReconciler(mgr, of,
			providerconfig.WithLogger(o.Logger.WithValues("controller", name)),
			providerconfig.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))) //nolint:staticcheck // crossplane-runtime does not support new events API yet (crossplane/crossplane#7152)
}

// SetupGated adds a controller that reconciles ProviderConfigs by accounting for
// their current usage.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			mgr.GetLogger().Error(err, "unable to setup reconcilers", "gvk", pcv1beta1.ClusterProviderConfigGroupVersionKind.String(), "gvk", pcv1beta1.ProviderConfigGroupVersionKind.String())
		}
	}, pcv1beta1.ClusterProviderConfigGroupVersionKind, pcv1beta1.ProviderConfigGroupVersionKind, pcv1beta1.ProviderConfigUsageGroupVersionKind)
	return nil
}
