/*
Copyright 2025.

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

package controller

import (
	"context"
	"fmt"
	"time"

	opsv1alpha1 "bennsimon.me/configmirror-operator/api/v1alpha1"
	"bennsimon.me/configmirror-operator/internal/repository"
	"bennsimon.me/configmirror-operator/pkg/utils"
	"github.com/go-logr/logr"
	"github.com/jackc/pgx/v5/pgxpool"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ConfigMirrorReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Database *pgxpool.Pool
	repository.ConfigMapRepository
	repository.BaseRepository
}

// +kubebuilder:rbac:groups="",resources=events;configmaps,verbs=create;patch;update;get;list;watch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch

// +kubebuilder:rbac:groups=bennsimon.github.io,resources=configmirrors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=bennsimon.github.io,resources=configmirrors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=bennsimon.github.io,resources=configmirrors/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=configmaps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=configmaps/finalizers,verbs=update
// +kubebuilder:resource:scope=cluster

var log logr.Logger

func (r *ConfigMirrorReconciler) Initialize(ctx context.Context) error {
	log = ctrl.Log.WithName(utils.ControllerName)

	if utils.SaveReplicationAction() {
		err := r.InitializeDatabase(ctx)
		if err != nil {
			return err
		}
	} else {
		log.Info("saving replication action disabled")
	}

	return nil
}

func (r *ConfigMirrorReconciler) InitializeDatabase(ctx context.Context) error {
	dbCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	url, err := utils.GetDatabaseUrl()

	if err != nil {
		return err
	}
	pool, err := pgxpool.New(dbCtx, url)

	if err != nil {
		return err
	}

	log.Info("database connecting...")
	r.Database = pool

	r.BaseRepository = repository.NewBaseRepositoryImpl(r.Database)

	err = r.InitMigration(ctx)
	if err != nil {
		return err
	}
	log.Info("database migration completed...")

	r.ConfigMapRepository = repository.NewConfigMapRepositoryImpl(r.Database)
	return nil
}

func (r *ConfigMirrorReconciler) Shutdown() {
	if utils.SaveReplicationAction() {
		log.Info("shutting down database connection")
		r.Database.Close()
	}
}

func (r *ConfigMirrorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log = logf.FromContext(ctx)
	configMirror := &opsv1alpha1.ConfigMirror{}
	apiCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := r.Get(apiCtx, client.ObjectKey{Namespace: req.Namespace, Name: req.Name}, configMirror)

	if err != nil {
		log.Error(err, fmt.Sprintf("unable to fetch ConfigMirror %s/%s", req.Namespace, req.Name))
		return ctrl.Result{}, err
	}

	configmaps, err := r.fetchMatchingConfigMaps(ctx, configMirror)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info(fmt.Sprintf("configMaps for configmirror %s/%s not found", req.Namespace, req.Name))
		} else {
			log.Error(err, fmt.Sprintf("Failed to fetch ConfigMaps for ConfigMirror %s/%s", req.Namespace, req.Name))
		}
		return ctrl.Result{}, err
	}

	if len(configmaps.Items) > 0 {
		err := r.replicateConfigMaps(ctx, configMirror, configmaps)
		if err != nil {
			log.Error(err, "Failed to replicate ConfigMaps")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ConfigMirrorReconciler) replicateConfigMaps(ctx context.Context, configMirror *opsv1alpha1.ConfigMirror, configMaps *v1.ConfigMapList) error {
	targetNamespaces := configMirror.Spec.TargetNamespaces
	for _, targetNamespace := range *targetNamespaces {
		for _, configMap := range configMaps.Items {

			_configMap := configMap.DeepCopy()
			_configMap.SetNamespace(targetNamespace)
			_configMap.SetUID("")
			_configMap.SetResourceVersion("")
			_configMap.ManagedFields = nil
			delete(_configMap.Annotations, "kubectl.kubernetes.io/last-applied-configuration")

			if _configMap.Annotations == nil {
				_configMap.Annotations = map[string]string{}
			}

			r.updateConfigMapAnnotations(_configMap, configMap, configMirror)

			apiCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := r.Patch(apiCtx, _configMap, client.Apply, client.FieldOwner(utils.GetFieldOwner()))
			cancel()
			if err != nil {
				return fmt.Errorf("failed to replicate %s/%s to %s/%s due to: %v", configMap.GetNamespace(), configMap.GetName(), _configMap.GetNamespace(), _configMap.GetName(), err)
			}

			log.Info(fmt.Sprintf("Replicated ConfigMap %s/%s to %s/%s", configMap.GetNamespace(), configMap.GetName(), _configMap.GetNamespace(), _configMap.GetName()))

			if utils.SaveReplicationAction() {

				dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				err = r.AddOrUpdate(dbCtx, _configMap)
				cancel()
				if err != nil {
					return err
				}

				log.Info(fmt.Sprintf("ConfigMap %s/%s updated on database", _configMap.GetNamespace(), _configMap.GetName()))
			}
		}

	}
	return nil
}

func (r *ConfigMirrorReconciler) fetchMatchingConfigMaps(ctx context.Context, configMirror *opsv1alpha1.ConfigMirror) (*v1.ConfigMapList, error) {
	configmaps := &v1.ConfigMapList{}

	selector := &metav1.LabelSelector{
		MatchLabels: configMirror.Spec.Selector.MatchLabels,
	}
	asSelector, err := metav1.LabelSelectorAsSelector(selector)

	if err != nil {
		return nil, err
	}

	apiCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err = r.List(apiCtx, configmaps, client.InNamespace(*configMirror.Spec.SourceNamespace),
		client.MatchingLabelsSelector{Selector: asSelector})

	if err != nil {
		return nil, err
	}

	log.Info(fmt.Sprintf("Found %d ConfigMaps matching selector %s in %s namespace", len(configmaps.Items), *selector, *configMirror.Spec.SourceNamespace))
	return configmaps, nil
}

func (r *ConfigMirrorReconciler) updateConfigMapAnnotations(_configMap *v1.ConfigMap, configMap v1.ConfigMap, configMirror *opsv1alpha1.ConfigMirror) {
	_configMap.Annotations[utils.GetAnnotationKey(utils.MirrorKey)] = string(configMap.GetUID())
	_configMap.Annotations[utils.GetAnnotationKey(utils.SourceNamespace)] = configMap.Namespace
	_configMap.Annotations[utils.GetAnnotationKey(utils.ManagedBy)] = fmt.Sprintf("%s/%s", configMirror.GetNamespace(), configMirror.GetName())
}

func (r *ConfigMirrorReconciler) EnqueueRequest(ctx context.Context, obj client.Object, mgr ctrl.Manager) []reconcile.Request {
	var reqs []reconcile.Request

	if _, ok := obj.GetAnnotations()[utils.GetAnnotationKey(utils.MirrorKey)]; ok {
		configMap := &v1.ConfigMap{}
		apiCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		err := mgr.GetClient().Get(apiCtx, client.ObjectKey{Namespace: obj.GetNamespace(), Name: obj.GetName()}, configMap)

		if err != nil && errors.IsNotFound(err) {
			_sourceNamespace := obj.GetAnnotations()[utils.GetAnnotationKey(utils.SourceNamespace)]
			log.Info(fmt.Sprintf("Detected deleted replicated configmap %s/%s, recreating if calling configmirror exists", obj.GetNamespace(), obj.GetName()))
			obj.SetNamespace(_sourceNamespace)
		} else if err != nil {
			log.Error(err, fmt.Sprintf("Error occurred while recreating deleted configmap %s/%s", obj.GetNamespace(), obj.GetName()))
			return reqs
		} else {
			log.Info(fmt.Sprintf("Configmap %s/%s is already replicated, skipping", obj.GetNamespace(), obj.GetName()))
			return reqs
		}
	}

	var configMirrorList opsv1alpha1.ConfigMirrorList
	apiCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_ = mgr.GetClient().List(apiCtx, &configMirrorList)
	for _, configMirror := range configMirrorList.Items {
		if obj.GetNamespace() == *configMirror.Spec.SourceNamespace {
			configMirrorSelector := &metav1.LabelSelector{
				MatchLabels: configMirror.Spec.Selector.MatchLabels,
			}
			selector, _ := metav1.LabelSelectorAsSelector(configMirrorSelector)
			if selector.Matches(labels.Set(obj.GetLabels())) {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: configMirror.Name, Namespace: configMirror.Namespace,
					},
				})
			}
		}
	}
	return reqs
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigMirrorReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&opsv1alpha1.ConfigMirror{}).
		Watches(
			&v1.ConfigMap{},
			handler.TypedEnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
				return r.EnqueueRequest(ctx, obj, mgr)
			}),
		).
		Named(utils.ControllerName).
		Complete(r)
}
