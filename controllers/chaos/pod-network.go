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
	"strings"
	"time"
)

func podNetworkJob(chaosJobName string, step v1alpha1.ChaosStep, generation int64, nodeName, podName string) batchV1.Job {
	var envs []coreV1.EnvVar
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
			BackoffLimit: utils.PtrData(int32(0)),
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
					HostPID:                       true,
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
						{
							Name: "sys-path",
							VolumeSource: coreV1.VolumeSource{
								HostPath: &coreV1.HostPathVolumeSource{
									Path: "/sys",
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
								{
									Name:      "sys-path",
									MountPath: "/sys",
								},
							},
							Command:         []string{"/bin/bash"},
							Args:            []string{"-c", "./helpers -name helm-network-chaos"},
							Name:            step.Name,
							Image:           step.Image,
							Env:             envs,
							Resources:       coreV1.ResourceRequirements{},
							ImagePullPolicy: coreV1.PullAlways,
							SecurityContext: &coreV1.SecurityContext{
								Privileged: utils.PtrData(true),
								RunAsUser:  utils.PtrData(int64(0)),
								Capabilities: &coreV1.Capabilities{
									Add: []coreV1.Capability{
										"SYS_ADMIN",
										"NET_ADMIN",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return job
}

func runNetworkChaos(chaosJobName string, step v1alpha1.ChaosStep, generation int64) {
	start := time.Now().Unix()
	duration, _ := strconv.Atoi(step.Config["TOTAL_CHAOS_DURATION"])
	elapsed := int(start) + duration
	logrus.Infof("creating step %s", step.Name)
	pods, err := utils.FilterTargetPods(step.Config)
	if err != nil {
		UpdateSnapshot(chaosJobName, step.Name, err.Error(), generation, v1alpha1.FailedStatus)
		UpdateJobStatus(fmt.Sprintf("%s-%v", chaosJobName, generation), "", v1alpha1.FailedStatus)
		return
	}
	for i := range pods {
		// populate the first container name if APP_CONTAINER is empty
		step.Config["APP_POD"] = pods[i].Name
		if step.Config["APP_CONTAINER"] == "" {
			step.Config["APP_CONTAINER"] = pods[i].Spec.Containers[0].Name
		}
		// lookup hosts here
		if step.Config["DESTINATION_HOSTS"] != "" {
			for _, hostname := range strings.Split(step.Config["DESTINATION_HOSTS"], ",") {
				hostname = strings.TrimSpace(hostname)
				addrs, err := utils.LookUpHost(hostname)
				if err != nil {
					UpdateSnapshot(chaosJobName, step.Name, err.Error(), generation, v1alpha1.FailedStatus)
					UpdateJobStatus(fmt.Sprintf("%s-%v", chaosJobName, generation), err.Error(), v1alpha1.FailedStatus)
					return
				}
				if step.Config["DESTINATION_IPS"] == "" {
					for _, addr := range addrs {
						step.Config["DESTINATION_IPS"] += addr
					}
				} else {
					for _, addr := range addrs {
						step.Config["DESTINATION_IPS"] += fmt.Sprintf(",%s", addr)
					}
				}
			}
		}

		job := podNetworkJob(chaosJobName, step, generation, pods[i].Spec.NodeName, pods[i].Name)
		_, err := env.KubeClient.BatchV1().Jobs(env.JobNamespace).Create(context.TODO(), &job, metaV1.CreateOptions{})
		if err != nil {
			// update status
			UpdateSnapshot(chaosJobName, step.Name, err.Error(), generation, v1alpha1.FailedStatus)
			UpdateJobStatus(fmt.Sprintf("%s-%v", chaosJobName, generation), "", v1alpha1.FailedStatus)
			return
		}
	}
	logrus.Infof("pods for step %s created", step.Name)
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
