/*
Copyright 2023 kbfu.

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
	"fmt"
	"github.com/kbfu/godzilla-operator/api/v1alpha1"
	"github.com/kbfu/godzilla-operator/controllers/chaos"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sync"
)

// GodzillaJobReconciler reconciles a GodzillaJob object
type GodzillaJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=charts-chaos.io,resources=godzillajobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=charts-chaos.io,resources=godzillajobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=charts-chaos.io,resources=godzillajobs/finalizers,verbs=update
//+kubebuilder:rbac:groups=charts-chaos.io,resources=godzillajobsnapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=charts-chaos.io,resources=godzillajobsnapshots/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=charts-chaos.io,resources=godzillajobsnapshots/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// Modify the Reconcile function to compare the state specified by
// the GodzillaJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *GodzillaJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var job v1alpha1.GodzillaJob
	err := r.Get(ctx, req.NamespacedName, &job)
	if err != nil {
		logrus.Error(err)
		return ctrl.Result{}, err
	}
	// run all inside scenarios
	chaos.OverrideConfig(job.Spec.Steps)

	err = chaos.InitSnapshot(job)
	if err != nil {
		logrus.Error(err)
		return ctrl.Result{}, err
	}
	// pre-check before run
	err = chaos.PreCheck(job)
	if err != nil {
		logrus.Error(err)
		return ctrl.Result{}, err
	}

	go func() {
		var wg sync.WaitGroup
		chaos.UpdateJobStatus(fmt.Sprintf("%s-%v", job.Name, job.Generation),
			"", v1alpha1.RunningStatus)
		for _, steps := range job.Spec.Steps {
			for _, s := range steps {
				wg.Add(1)
				s := s
				go func() {
					logrus.Infof("running job %s", job.Name)
					chaos.Run(job.Name, s, job.ObjectMeta.Generation)
					wg.Done()
				}()
			}
			wg.Wait()
		}
		chaos.UpdateJobStatus(fmt.Sprintf("%s-%v", job.Name, job.Generation), "", v1alpha1.SuccessStatus)
	}()
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GodzillaJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.GodzillaJob{}).
		WithEventFilter(ignorePredicate()).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		Complete(r)
}

func ignorePredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.DeleteStateUnknown
		},
	}
}
