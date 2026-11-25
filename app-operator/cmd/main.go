package main

import (
	"os"

	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"

	apisv1alpha1 "github.com/kcp-dev/sdk/apis/apis/v1alpha1"
	corev1alpha1 "github.com/kcp-dev/sdk/apis/core/v1alpha1"
	tenancyv1alpha1 "github.com/kcp-dev/sdk/apis/tenancy/v1alpha1"

	"github.com/kcp-dev/multicluster-provider/apiexport"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"

	juiceshopv1alpha1 "github.com/platform-mesh/example-app-provider/api/v1alpha1"
	"github.com/platform-mesh/example-app-provider/internal/controller"
)

func init() {
	runtime.Must(corev1alpha1.AddToScheme(scheme.Scheme))
	runtime.Must(tenancyv1alpha1.AddToScheme(scheme.Scheme))
	runtime.Must(apisv1alpha1.AddToScheme(scheme.Scheme))
	runtime.Must(juiceshopv1alpha1.AddToScheme(scheme.Scheme))
	runtime.Must(cnpgv1.AddToScheme(scheme.Scheme))
}

func main() {
	log.SetLogger(zap.New(zap.UseDevMode(true)))

	ctx := signals.SetupSignalHandler()
	entryLog := log.Log.WithName("entrypoint")

	var (
		endpointSlice string
		provider      *apiexport.Provider
	)

	pflag.StringVar(&endpointSlice, "endpointslice", "examples-apiexport-multicluster", "Set the APIExportEndpointSlice name to watch")
	pflag.Parse()

	cfg := ctrl.GetConfigOrDie()

	entryLog.Info("Setting up manager")
	opts := manager.Options{}

	var err error
	provider, err = apiexport.New(cfg, endpointSlice, apiexport.Options{})
	if err != nil {
		entryLog.Error(err, "unable to construct cluster provider")
		os.Exit(1)
	}

	mgr, err := mcmanager.New(cfg, provider, opts)
	if err != nil {
		entryLog.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	if err := (&controller.AppReconciler{Manager: mgr}).SetupWithManager(mgr); err != nil {
		entryLog.Error(err, "failed to build controller", "controller", "App")
		os.Exit(1)
	}

	entryLog.Info("Starting manager")
	if err := mgr.Start(ctx); err != nil {
		entryLog.Error(err, "unable to run manager")
		os.Exit(1)
	}
}
