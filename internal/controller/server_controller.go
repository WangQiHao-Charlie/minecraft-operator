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

	minecraftv1 "github.com/WangQiHao-Charlie/minecraft-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServerReconciler reconciles a Server object
type ServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}
type Step func(ctx context.Context, server *minecraftv1.Server, r *ServerReconciler) error

// +kubebuilder:rbac:groups=minecraft.charlie-cloud.me,resources=servers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=minecraft.charlie-cloud.me,resources=servers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=minecraft.charlie-cloud.me,resources=servers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	var server minecraftv1.Server

	if err := r.Get(ctx, req.NamespacedName, &server); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	steps := []Step{
		r.ensureGameSetting,
		r.ensureServer,
	}

	for _, step := range steps {
		if err := step(ctx, &server, r); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&minecraftv1.Server{}).
		Owns(&v1.ConfigMap{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&v1.Service{}).
		Named("server").
		Complete(r)
}
