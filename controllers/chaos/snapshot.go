/*
 *
 * Copyright 2023 kbfu.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package chaos

import (
	"context"
	"fmt"
	"github.com/kbfu/godzilla-operator/api/v1alpha1"
	"github.com/kbfu/godzilla-operator/controllers/env"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func statusCheck(prev v1alpha1.JobStatus, curr v1alpha1.JobStatus) bool {
	if prev == v1alpha1.PendingStatus &&
		(curr == v1alpha1.RunningStatus || curr == v1alpha1.FailedStatus || curr == v1alpha1.UnknownStatus || curr == v1alpha1.SuccessStatus) {
		return true
	} else if prev == v1alpha1.RunningStatus &&
		(curr == v1alpha1.FailedStatus || curr == v1alpha1.UnknownStatus || curr == v1alpha1.SuccessStatus) {
		return true
	} else if prev == v1alpha1.SuccessStatus && curr == v1alpha1.FailedStatus {
		return true
	}
	return false
}

func InitSnapshot(job v1alpha1.GodzillaJob) error {
	// create snapshot here
	metadata := job.ObjectMeta
	job.Namespace = env.JobNamespace
	snapshot := v1alpha1.GodzillaJobSnapshot{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%v", metadata.Name, metadata.Generation),
			Namespace: job.Namespace,
		},
		Spec: v1alpha1.GodzillaJobSnapshotSpec{},
	}
	var nestedSteps [][]v1alpha1.ChaosStepSnapshot
	for i := range job.Spec.Steps {
		var steps []v1alpha1.ChaosStepSnapshot
		for j := range job.Spec.Steps[i] {
			step := v1alpha1.ChaosStepSnapshot{
				Name:               job.Spec.Steps[i][j].Name,
				Type:               job.Spec.Steps[i][j].Type,
				Config:             job.Spec.Steps[i][j].Config,
				Image:              job.Spec.Steps[i][j].Image,
				ServiceAccountName: job.Spec.Steps[i][j].ServiceAccountName,
				Status:             v1alpha1.PendingStatus,
				FailedReason:       "",
			}
			steps = append(steps, step)
		}
		nestedSteps = append(nestedSteps, steps)
	}
	snapshot.Spec.Steps = nestedSteps
	err := env.Client.Create(context.TODO(), &snapshot)
	if err != nil {
		logrus.Error(err)
		return err
	}
	snapshot.Status.JobStatus = v1alpha1.PendingStatus
	err = env.Client.Status().Update(context.TODO(), &snapshot)
	if err != nil {
		logrus.Error(err)
		return err
	}
	return nil
}

func UpdateJobStatus(name, reason string, jobStatus v1alpha1.JobStatus) error {
	var snapshot v1alpha1.GodzillaJobSnapshot
	err := env.Client.Get(context.TODO(), client.ObjectKey{
		Namespace: env.JobNamespace,
		Name:      name,
	}, &snapshot)
	if err != nil {
		logrus.Error(err)
		return err
	}
	if statusCheck(snapshot.Status.JobStatus, jobStatus) {
		snapshot.Status.JobStatus = jobStatus
		snapshot.Status.FailedReason = reason
		err = env.Client.Status().Update(context.TODO(), &snapshot)
		if err != nil {
			logrus.Error(err)
			return err
		}
	}
	return nil
}

func UpdateSnapshot(jobName, stepName, reason string, generation int64, stepStatus v1alpha1.JobStatus) error {
	if stepStatus == v1alpha1.FailedStatus {
		logrus.Errorf("job %s, step %s, failed reason %s", jobName, stepName, reason)
	}
	name := fmt.Sprintf("%s-%v", jobName, generation)
	var snapshot v1alpha1.GodzillaJobSnapshot
	err := env.Client.Get(context.TODO(), client.ObjectKey{
		Namespace: env.JobNamespace,
		Name:      name,
	}, &snapshot)
	if err != nil {
		logrus.Error()
		return err
	}
	logrus.Infof("updating snapshot for %s", snapshot.Name)
out:
	for i := range snapshot.Spec.Steps {
		for j := range snapshot.Spec.Steps[i] {
			if snapshot.Spec.Steps[i][j].Name == stepName {
				if statusCheck(snapshot.Spec.Steps[i][j].Status, stepStatus) {
					snapshot.Spec.Steps[i][j].Status = stepStatus
					snapshot.Spec.Steps[i][j].FailedReason = reason
					break out
				}
			}
		}
	}
	err = env.Client.Update(context.TODO(), &snapshot)
	if err != nil {
		logrus.Error(err)
		return err
	}
	return nil
}
