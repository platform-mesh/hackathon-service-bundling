package controller

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mcbuilder "sigs.k8s.io/multicluster-runtime/pkg/builder"
	mcmanager "sigs.k8s.io/multicluster-runtime/pkg/manager"
	mcreconcile "sigs.k8s.io/multicluster-runtime/pkg/reconcile"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"

	juiceshopv1alpha1 "github.com/platform-mesh/example-app-provider/api/v1alpha1"
)

// AppReconciler reconciles App objects across all engaged clusters.
type AppReconciler struct {
	Manager mcmanager.Manager
}

// Reconcile moves the current state of an App closer to the desired state.
func (r *AppReconciler) Reconcile(ctx context.Context, req mcreconcile.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithValues("cluster", req.ClusterName, "namespace", req.Namespace)

	cl, err := r.Manager.GetCluster(ctx, req.ClusterName)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get cluster: %w", err)
	}
	client := cl.GetClient()

	app := &juiceshopv1alpha1.App{}
	if err := client.Get(ctx, req.NamespacedName, app); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info(fmt.Sprintf("app %s got deleted", req.Name))
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("failed to get app: %w", err)
	}

	db := &cnpgv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
		},
		Spec: cnpgv1.ClusterSpec{
			Instances: 1,
			StorageConfiguration: cnpgv1.StorageConfiguration{
				Size: "1Gi",
			},
		},
	}

	switch err := client.Create(ctx, db); {
	case err == nil:
		log.Info(fmt.Sprintf("created database cluster %s for app %s", db.Name, app.Name))
	case apierrors.IsAlreadyExists(err):
		// Nothing to do, the cluster is only ever created, for now
	default:
		return reconcile.Result{}, fmt.Errorf("failed to create database cluster: %w", err)
	}

	// simply wait until the database secret is available
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{Namespace: app.Namespace, Name: db.Name + "-app"}
	if err := client.Get(ctx, secretKey, secret); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info(fmt.Sprintf("database secret %s not available yet", secretKey.Name))
			return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
		}
		return reconcile.Result{}, fmt.Errorf("failed to get database secret: %w", err)
	}

	fmt.Printf("database secret %s for app %s:\n", secret.Name, app.Name)
	for _, key := range slices.Sorted(maps.Keys(secret.Data)) {
		fmt.Printf("  %s: %s\n", key, strings.TrimSpace(string(secret.Data[key])))
	}

	log.Info(fmt.Sprintf("processed app %s", app.Name))

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the multicluster Manager.
func (r *AppReconciler) SetupWithManager(mgr mcmanager.Manager) error {
	return mcbuilder.ControllerManagedBy(mgr).
		Named("app-controller").
		For(&juiceshopv1alpha1.App{}).
		Complete(r)
}
