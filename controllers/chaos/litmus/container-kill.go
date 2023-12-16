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

package litmus

import (
	"context"
	"fmt"
	"github.com/kbfu/godzilla-operator/api/v1alpha1"
	"github.com/kbfu/godzilla-operator/controllers/chaos"
	"github.com/kbfu/godzilla-operator/controllers/chaos/litmus/utils"
	"github.com/kbfu/godzilla-operator/controllers/env"
	"github.com/sirupsen/logrus"
	batchV1 "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"strconv"
	"time"
)

func runContainerKill(chaosJobName string, step v1alpha1.ChaosStep, generation int64) {
	start := time.Now().Unix()
	duration, _ := strconv.Atoi(step.Config["TOTAL_CHAOS_DURATION"])
	elapsed := int(start) + duration
	logrus.Infof("creating step %s", step.Name)
	pods, err := utils.FilterTargetPods(step.Config)
	if err != nil {
		chaos.UpdateSnapshot(chaosJobName, step.Name, err.Error(), generation, v1alpha1.FailedStatus)
		chaos.UpdateJobStatus(fmt.Sprintf("%s-%v", chaosJobName, generation), "", v1alpha1.FailedStatus)
		return
	}
	for i := range pods {
		// populate the first container name if APP_CONTAINER is empty
		if step.Config["APP_CONTAINER"] == "" {
			step.Config["APP_CONTAINER"] = pods[i].Spec.Containers[0].Name
		}
		step.Config["APP_POD"] = pods[i].Name
		job := containerKillJob(chaosJobName, step, generation, pods[i].Spec.NodeName, pods[i].Name)
		_, err := chaos.KubeClient.BatchV1().Jobs(env.JobNamespace).Create(context.TODO(), &job, metaV1.CreateOptions{})
		if err != nil {
			// update status
			chaos.UpdateSnapshot(chaosJobName, step.Name, err.Error(), generation, v1alpha1.FailedStatus)
			chaos.UpdateJobStatus(fmt.Sprintf("%s-%v", chaosJobName, generation), "", v1alpha1.FailedStatus)
			return
		}
	}
	logrus.Infof("pods for step %s created", step.Name)

	// watch for the status
	w, err := chaos.KubeClient.CoreV1().Pods(env.JobNamespace).Watch(context.TODO(), metaV1.ListOptions{
		LabelSelector: fmt.Sprintf("chaos.job.generation=%v,chaos.step.name=%s,chaos.job.name=%s", generation, step.Name, chaosJobName),
	})
	if err != nil {
		logrus.Errorf("step %s status watch failed, reason: %s", step.Name, err.Error())
		// update status
		chaos.UpdateSnapshot(chaosJobName, step.Name, err.Error(), generation, v1alpha1.FailedStatus)
		chaos.UpdateJobStatus(fmt.Sprintf("%s-%v", chaosJobName, generation), "", v1alpha1.FailedStatus)
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
					err = chaos.CleanJob(chaosJobName, step, generation)
					if err != nil {
						logrus.Errorf("step %s cleanup failed, reason: %s", step.Name, err.Error())
						// update status
						chaos.UpdateSnapshot(chaosJobName, step.Name, err.Error(), generation, v1alpha1.FailedStatus)
						chaos.UpdateJobStatus(fmt.Sprintf("%s-%v", chaosJobName, generation), "", v1alpha1.FailedStatus)
						w.Stop()
						return
					}
					logrus.Infof("step %s cleanup done", step.Name)
					switch podObject.Status.Phase {
					case coreV1.PodSucceeded:
						// update status
						chaos.UpdateSnapshot(chaosJobName, step.Name, "", generation, v1alpha1.SuccessStatus)
					case coreV1.PodFailed:
						// update status
						chaos.UpdateSnapshot(chaosJobName, step.Name, "chaos step pod running failed", generation,
							v1alpha1.FailedStatus)
						chaos.UpdateJobStatus(fmt.Sprintf("%s-%v", chaosJobName, generation), "", v1alpha1.FailedStatus)
					default:
						// update status
						chaos.UpdateSnapshot(chaosJobName, step.Name, "", generation, v1alpha1.UnknownStatus)
						chaos.UpdateJobStatus(fmt.Sprintf("%s-%v", chaosJobName, generation), "", v1alpha1.UnknownStatus)
					}
					w.Stop()
					break
				} else {
					// timeout, mark to failed status
					if elapsed+120 < int(time.Now().Unix()) {
						// update status
						chaos.UpdateSnapshot(chaosJobName, step.Name, "chaos step pod not started", generation, v1alpha1.FailedStatus)
						chaos.UpdateJobStatus(fmt.Sprintf("%s-%v", chaosJobName, generation), "", v1alpha1.FailedStatus)
						logrus.Infof("step %s failed, starting cleanup", step.Name)
						chaos.CleanJob(chaosJobName, step, generation)
						w.Stop()
						break
					}
				}
			}
		}
	}
}

func containerKillJob(chaosJobName string, step v1alpha1.ChaosStep, generation int64, nodeName, podName string) batchV1.Job {
	var (
		backOffLimit int32 = 0
		envs         []coreV1.EnvVar
		privileged   = true
	)

	termination, _ := strconv.ParseInt(step.Config["TERMINATION_GRACE_PERIOD_SECONDS"], 10, 64)
	jobName := fmt.Sprintf("%s-%s", step.Name, utils.RandomString(10))

	// setup env vars
	for k, v := range step.Config {
		envs = append(envs, coreV1.EnvVar{
			Name:  k,
			Value: v,
		})
	}

	job := batchV1.Job{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      jobName,
			Namespace: env.JobNamespace,
			Labels: map[string]string{
				"chaos.job.generation": fmt.Sprintf("%v", generation),
				"chaos.step.name":      step.Name,
				"chaos.job.pod":        podName,
				"chaos.job.name":       chaosJobName,
			},
		},
		Spec: batchV1.JobSpec{
			BackoffLimit: &backOffLimit,
			Template: coreV1.PodTemplateSpec{
				ObjectMeta: metaV1.ObjectMeta{
					Labels: map[string]string{
						"chaos.job.generation": fmt.Sprintf("%v", generation),
						"chaos.step.name":      step.Name,
						"chaos.job.pod":        podName,
						"chaos.job.name":       chaosJobName,
					},
				},
				Spec: coreV1.PodSpec{
					TerminationGracePeriodSeconds: &termination,
					NodeName:                      nodeName,
					Volumes: []coreV1.Volume{
						{
							Name: "socket-path",
							VolumeSource: coreV1.VolumeSource{
								HostPath: &coreV1.HostPathVolumeSource{
									Path: step.Config["SOCKET_PATH"],
								},
							},
						},
					},
					ServiceAccountName: step.ServiceAccountName,
					RestartPolicy:      coreV1.RestartPolicyNever,
					Containers: []coreV1.Container{
						{
							VolumeMounts: []coreV1.VolumeMount{
								{
									Name:      "socket-path",
									MountPath: step.Config["SOCKET_PATH"],
								},
							},
							Command:         []string{"/bin/bash"},
							Args:            []string{"-c", "./helpers -name container-kill"},
							Name:            step.Name,
							Image:           step.Image,
							Env:             envs,
							Resources:       coreV1.ResourceRequirements{},
							ImagePullPolicy: coreV1.PullAlways,
							SecurityContext: &coreV1.SecurityContext{
								Privileged: &privileged,
							},
						},
					},
				},
			},
		},
	}
	return job
}