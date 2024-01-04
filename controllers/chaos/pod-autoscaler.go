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
	"github.com/kbfu/godzilla-operator/controllers/chaos/utils"
	"github.com/kbfu/godzilla-operator/controllers/env"
	"github.com/sirupsen/logrus"
	batchV1 "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"strconv"
	"time"
)

func podAutoscalerJob(chaosJobName string, step v1alpha1.ChaosStep, generation int64) batchV1.Job {
	var envs []coreV1.EnvVar

	// setup env vars
	for k, v := range step.Config {
		envs = append(envs, coreV1.EnvVar{
			Name:  k,
			Value: v,
		})
	}

	job := batchV1.Job{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      step.Name,
			Namespace: env.JobNamespace,
			Labels: map[string]string{
				"chaos.job.generation": fmt.Sprintf("%v", generation),
				"chaos.step.name":      step.Name,
				"chaos.job.name":       chaosJobName,
			},
		},
		Spec: batchV1.JobSpec{
			BackoffLimit: utils.PtrData(int32(0)),
			Template: coreV1.PodTemplateSpec{
				ObjectMeta: metaV1.ObjectMeta{
					Labels: map[string]string{
						"chaos.job.generation": fmt.Sprintf("%v", generation),
						"chaos.step.name":      step.Name,
						"chaos.job.name":       chaosJobName,
					},
				},
				Spec: coreV1.PodSpec{
					ServiceAccountName: step.ServiceAccountName,
					RestartPolicy:      coreV1.RestartPolicyNever,
					Containers: []coreV1.Container{
						{
							Command:         []string{"/bin/bash"},
							Args:            []string{"-c", "./helpers -name helm-pod-autoscaler"},
							Name:            step.Name,
							Image:           step.Image,
							Env:             envs,
							Resources:       coreV1.ResourceRequirements{},
							ImagePullPolicy: coreV1.PullAlways,
							SecurityContext: &coreV1.SecurityContext{
								Privileged: utils.PtrData(false),
							},
						},
					},
				},
			},
		},
	}
	return job
}

func runPodAutoscaler(chaosJobName string, step v1alpha1.ChaosStep, generation int64) {
	job := podAutoscalerJob(chaosJobName, step, generation)
	logrus.Infof("creating step %s", step.Name)
	start := time.Now().Unix()
	duration, _ := strconv.Atoi(step.Config["TOTAL_CHAOS_DURATION"])
	elapsed := int(start) + duration
	_, err := env.KubeClient.BatchV1().Jobs(env.JobNamespace).Create(context.TODO(), &job, metaV1.CreateOptions{})
	if err != nil {
		logrus.Errorf("step %s run failed, reason: %s", step.Name, err.Error())
		// update status
		UpdateSnapshot(chaosJobName, step.Name, err.Error(), generation, v1alpha1.FailedStatus)
		UpdateJobStatus(chaosJobName, "", v1alpha1.FailedStatus)
		return
	}
	logrus.Infof("step %s created", step.Name)
	// status started
	// update status
	UpdateSnapshot(chaosJobName, step.Name, "", generation, v1alpha1.RunningStatus)
	// watch for the status
	w, err := env.KubeClient.CoreV1().Pods(env.JobNamespace).Watch(context.TODO(), metaV1.ListOptions{
		LabelSelector: fmt.Sprintf("chaos.job.generation=%v,chaos.step.name=%s,chaos.job.name=%s", generation, step.Name, chaosJobName),
	})
	if err != nil {
		logrus.Errorf("step %s status watch failed, reason: %s", step.Name, err.Error())
		// update status
		UpdateSnapshot(chaosJobName, step.Name, err.Error(), generation, v1alpha1.FailedStatus)
		UpdateJobStatus(fmt.Sprintf("%s-%v", chaosJobName, generation), "", v1alpha1.FailedStatus)
		return
	}
	logrus.Infof("watching for step %s", step.Name)
	for event := range w.ResultChan() {
		if event.Object != nil {
			if reflect.ValueOf(event.Object).Type().Elem().Name() == "Pod" {
				podObject := event.Object.(*coreV1.Pod)
				if podObject.Status.Phase == coreV1.PodSucceeded || podObject.Status.Phase == coreV1.PodFailed {
					// cleanup
					logrus.Infof("step %s finished, starting cleanup", step.Name)
					err = CleanJob(chaosJobName, step, generation)
					if err != nil {
						logrus.Errorf("step %s cleanup failed, reason: %s", step.Name, err.Error())
						// update status
						UpdateSnapshot(chaosJobName, step.Name, err.Error(), generation, v1alpha1.FailedStatus)
						UpdateJobStatus(fmt.Sprintf("%s-%v", chaosJobName, generation), "", v1alpha1.FailedStatus)
						w.Stop()
						return
					}
					logrus.Infof("step %s cleanup done", step.Name)
					switch podObject.Status.Phase {
					case coreV1.PodSucceeded:
						// update status
						UpdateSnapshot(chaosJobName, step.Name, "", generation, v1alpha1.SuccessStatus)
					case coreV1.PodFailed:
						// update status
						UpdateSnapshot(chaosJobName, step.Name, "chaos step pod running failed", generation,
							v1alpha1.FailedStatus)
						UpdateJobStatus(fmt.Sprintf("%s-%v", chaosJobName, generation), "", v1alpha1.FailedStatus)
					default:
						// update status
						UpdateSnapshot(chaosJobName, step.Name, "", generation, v1alpha1.UnknownStatus)
						UpdateJobStatus(fmt.Sprintf("%s-%v", chaosJobName, generation), "", v1alpha1.UnknownStatus)
					}
					w.Stop()
					break
				} else {
					// timeout, mark to failed status
					if elapsed+120 < int(time.Now().Unix()) {
						// update status
						UpdateSnapshot(chaosJobName, step.Name, "chaos step pod not started", generation, v1alpha1.FailedStatus)
						UpdateJobStatus(fmt.Sprintf("%s-%v", chaosJobName, generation), "", v1alpha1.FailedStatus)
						logrus.Infof("step %s failed, starting cleanup", step.Name)
						CleanJob(chaosJobName, step, generation)
						w.Stop()
						break
					}
				}
			}
		}
	}
}
