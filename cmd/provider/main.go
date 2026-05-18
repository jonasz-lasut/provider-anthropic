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

// Package main is the entry point for the provider binary.
package main

import (
	"os"
	"path/filepath"
	"time"

	"github.com/alecthomas/kingpin/v2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/gate"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/customresourcesgate"

	"github.com/jonasz-lasut/provider-anthropic-platform/apis"
	xpcontroller "github.com/jonasz-lasut/provider-anthropic-platform/internal/controller"
)

func main() {
	app := kingpin.New(filepath.Base(os.Args[0]), "Anthropic platform support for Crossplane.").DefaultEnvars()
	skipDefaultMetadata := app.Flag(
		"skip-default-metadata",
		"Do not set default Crossplane identifiers on spec.forProvider.metadata.",
	).Bool()
	kingpin.MustParse(app.Parse(os.Args[1:]))

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	log := logging.NewLogrLogger(ctrl.Log.WithName(filepath.Base(os.Args[0])))

	// Omitting Scheme makes controller-runtime fall back to client-go's default
	// scheme, which is pre-populated with all standard Kubernetes types
	// (corev1, appsv1, rbacv1, ...). The credential extractor relies on
	// corev1.Secret being registered.
	syncPeriod := 10 * time.Minute
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
		},
		LeaderElection:   true,
		LeaderElectionID: "provider-anthropic-platform.crossplane.io",
	})
	if err != nil {
		log.Info("Cannot create manager", "error", err)
		os.Exit(1)
	}

	// Layer provider-specific types onto the default scheme.
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Info("Cannot register API types", "error", err)
		os.Exit(1)
	}
	// Required for the customresourcesgate controller to watch CRD objects.
	if err := apiextensionsv1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Info("Cannot register apiextensions scheme", "error", err)
		os.Exit(1)
	}

	g := new(gate.Gate[schema.GroupVersionKind])

	o := controller.Options{
		Logger:                  log,
		GlobalRateLimiter:       ratelimiter.NewGlobal(1),
		PollInterval:            1 * time.Minute,
		MaxConcurrentReconciles: 1,
		Features:                &feature.Flags{},
		Gate:                    g,
	}

	// Start the CRD gate controller — it watches CustomResourceDefinitions and
	// calls g.Set(gvk, true) when each CRD becomes established, unblocking the
	// corresponding SetupGated callbacks.
	if err := customresourcesgate.Setup(mgr, o); err != nil {
		log.Info("Cannot setup customresourcesgate controller", "error", err)
		os.Exit(1)
	}

	if err := xpcontroller.SetupProviders(mgr, o, *skipDefaultMetadata); err != nil {
		log.Info("Cannot setup controllers", "error", err)
		os.Exit(1)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Info("Manager exited with error", "error", err)
		os.Exit(1)
	}
}
