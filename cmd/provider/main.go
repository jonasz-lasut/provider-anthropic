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

// Package main is the entry point for the provider binary.
package main

import (
	"os"
	"path/filepath"
	"time"

	"github.com/alecthomas/kingpin/v2"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/gate"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/customresourcesgate"

	"github.com/jonasz-lasut/provider-anthropic/apis"
	xpcontroller "github.com/jonasz-lasut/provider-anthropic/internal/controller"
)

func main() {
	app := kingpin.New(filepath.Base(os.Args[0]), "Anthropic platform support for Crossplane.").DefaultEnvars()
	debug := app.Flag("debug", "Run with debug logging.").Short('d').Bool()
	skipDefaultMetadata := app.Flag(
		"skip-default-metadata",
		"Do not set default Crossplane identifiers on spec.forProvider.metadata.",
	).Bool()
	enableSecretCache := app.Flag(
		"enable-secret-cache",
		"Cache Secrets in the informer cache. Disable to read Secrets live from the "+
			"API server instead, trading reconcile QPS for lower memory use.",
	).Default("true").Envar("ENABLE_SECRET_CACHE").Bool()
	kingpin.MustParse(app.Parse(os.Args[1:]))

	zl := zap.New(zap.UseDevMode(*debug))
	if *debug {
		ctrl.SetLogger(zl)
	}
	log := logging.NewLogrLogger(zl.WithName(filepath.Base(os.Args[0])))

	// Built explicitly (rather than relying on the manager's default scheme
	// plus later AddToScheme calls on mgr.GetScheme()) so it's fully
	// populated before the manager is created. Includes client-go's default
	// types (corev1, appsv1, rbacv1, ...) since the credential extractor
	// relies on corev1.Secret being registered.
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		log.Info("Cannot register client-go scheme", "error", err)
		os.Exit(1)
	}
	if err := apis.AddToScheme(scheme); err != nil {
		log.Info("Cannot register API types", "error", err)
		os.Exit(1)
	}
	// Required for the customresourcesgate controller to watch CRD objects.
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		log.Info("Cannot register apiextensions scheme", "error", err)
		os.Exit(1)
	}

	var clientOpts client.Options
	if !*enableSecretCache {
		clientOpts = client.Options{
			Cache: &client.CacheOptions{DisableFor: []client.Object{&corev1.Secret{}}},
		}
	}

	syncPeriod := 10 * time.Minute
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Client: clientOpts,
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
			ByObject: map[client.Object]cache.ByObject{
				// Strips the OpenAPI schema, managedFields, and the
				// last-applied-configuration annotation from CRDs before they
				// enter the informer cache
				&apiextensionsv1.CustomResourceDefinition{}: {
					Transform: customresourcesgate.TransformStripCRDSchema,
				},
			},
		},
		LeaderElection:   true,
		LeaderElectionID: "provider-anthropic.crossplane.io",
	})
	if err != nil {
		log.Info("Cannot create manager", "error", err)
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
