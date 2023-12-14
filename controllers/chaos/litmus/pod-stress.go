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
	"k8s.io/apimachinery/pkg/watch"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func podStressJob(chaosJobName string, step v1alpha1.ChaosStep, generation int64, nodeName, podName string) batchV1.Job {
	var (
		backOffLimit int32 = 0
		envs         []coreV1.EnvVar
		privileged         = true
		user         int64 = 0
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
							Args:            []string{"-c", "./helpers -name stress-chaos"},
							Name:            step.Name,
							Image:           step.Image,
							Env:             envs,
							Resources:       coreV1.ResourceRequirements{},
							ImagePullPolicy: coreV1.PullAlways,
							SecurityContext: &coreV1.SecurityContext{
								Privileged: &privileged,
								RunAsUser:  &user,
								Capabilities: &coreV1.Capabilities{
									Add: []coreV1.Capability{
										"SYS_ADMIN",
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

func runPodStress(chaosJobName string, step v1alpha1.ChaosStep, generation int64) {
	var job batchV1.Job
	start := time.Now().Unix()
	duration, _ := strconv.Atoi(step.Config["TOTAL_CHAOS_DURATION"])
	elapsed := int(start) + duration
	go func() {
		logrus.Infof("creating step %s", step.Name)
		var targetPods []string
		if step.Config["TARGET_PODS"] != "" {
			targetPods = strings.Split(step.Config["TARGET_PODS"], ",")
		}
		var percentage int
		if step.Config["PODS_AFFECTED_PERC"] == "" {
			percentage = 0
		} else {
			percentage, _ = strconv.Atoi(step.Config["PODS_AFFECTED_PERC"])
		}

		var (
			pods    []coreV1.Pod
			allPods []coreV1.Pod
		)

		// if TARGET_PODS is not empty, use it
		if len(targetPods) > 0 {
			for _, targetPod := range targetPods {
				targetPod = strings.TrimSpace(targetPod)
				podObject, err := chaos.KubeClient.CoreV1().Pods(step.Config["APP_NAMESPACE"]).Get(context.TODO(), targetPod, metaV1.GetOptions{})
				if err != nil {
					// update status
					chaos.UpdateSnapshot(chaosJobName, step.Name, err.Error(), generation, v1alpha1.FailedStatus)
					chaos.UpdateJobStatus(chaosJobName, "", v1alpha1.FailedStatus)
					return
				}
				pods = append(pods, *podObject)
			}
		} else {
			// check label
			podList, err := chaos.KubeClient.CoreV1().Pods(step.Config["APP_NAMESPACE"]).List(context.TODO(), metaV1.ListOptions{
				LabelSelector: step.Config["APP_LABEL"],
				FieldSelector: "status.phase=Running",
			})
			if err != nil {
				// update status
				chaos.UpdateSnapshot(chaosJobName, step.Name, err.Error(), generation, v1alpha1.FailedStatus)
				chaos.UpdateJobStatus(chaosJobName, "", v1alpha1.FailedStatus)
				return
			}
			for i := range podList.Items {
				for _, condition := range podList.Items[i].Status.Conditions {
					if condition.Type == coreV1.ContainersReady && condition.Status == coreV1.ConditionTrue {
						if podList.Items[i].ObjectMeta.DeletionTimestamp == nil {
							allPods = append(allPods, podList.Items[i])
							pods = append(pods, podList.Items[i])
						}
					}
				}
			}
		}
		logrus.Infof("the total running pods is %d", len(allPods))
		// set the pods as percentage
		if percentage == 0 {
			pods = pods[:1]
		} else {
			pods = pods[:len(pods)*percentage/100]
		}
		var podNames []string
		for i := range pods {
			podNames = append(podNames, pods[i].Name)
		}
		logrus.Infof("the target pods are %v", podNames)

		for _, podObject := range pods {
			if podObject.Status.Phase == coreV1.PodRunning {
				// need to fetch the target node name
				nodeName := podObject.Spec.NodeName
				podName := podObject.Name
				step.Config["APP_POD"] = podName
				step.Config["CPU_CORES"] = "0"
				if step.Config["APP_CONTAINER"] == "" {
					step.Config["APP_CONTAINER"] = podObject.Spec.Containers[0].Name
				}
				step.Config["FILESYSTEM_UTILIZATION_PERCENTAGE"] = "0"
				step.Config["STRESS_TYPE"] = "pod-io-stress"
				job = podStressJob(chaosJobName, step, generation, nodeName, podName)
				_, err := chaos.KubeClient.BatchV1().Jobs(env.JobNamespace).Create(context.TODO(), &job, metaV1.CreateOptions{})
				if err != nil {
					// update status
					chaos.UpdateSnapshot(chaosJobName, step.Name, err.Error(), generation, v1alpha1.FailedStatus)
					chaos.UpdateJobStatus(chaosJobName, "", v1alpha1.FailedStatus)
					return
				}
			}
		}
		logrus.Infof("step %s created", step.Name)

		// step status started
		chaos.UpdateSnapshot(chaosJobName, step.Name, "", generation, v1alpha1.RunningStatus)

		// only label pods needs to be watched
		w, err := chaos.KubeClient.CoreV1().Pods(step.Config["APP_NAMESPACE"]).Watch(context.TODO(), metaV1.ListOptions{
			LabelSelector: step.Config["APP_LABEL"],
		})
		if err != nil {
			// update status
			chaos.UpdateSnapshot(chaosJobName, step.Name, err.Error(), generation, v1alpha1.FailedStatus)
			chaos.UpdateJobStatus(chaosJobName, "", v1alpha1.FailedStatus)
			return
		}
		logrus.Infof("watching for step %s", step.Name)

		for event := range w.ResultChan() {
			// check timeout
			if elapsed < int(time.Now().Unix()) {
				logrus.Infof("watch for step %s ended", step.Name)
				w.Stop()
				return
			}

			if event.Object != nil {
				if reflect.ValueOf(event.Object).Type().Elem().Name() == "Pod" {
					podObject := event.Object.(*coreV1.Pod)
					switch event.Type {
					case watch.Modified:
						if podObject.Status.Phase == coreV1.PodRunning {
							for _, condition := range podObject.Status.Conditions {
								if condition.Type == coreV1.ContainersReady && condition.Status == coreV1.ConditionTrue {
									if podObject.ObjectMeta.DeletionTimestamp == nil {
										// detect newly ready pod
										// if the pod is in the list, just start a new chaos pod
										logrus.Infof("new running pod %s detected, step %s",
											podObject.Name, step.Name)
										for _, p := range pods {
											if p.Name == podObject.Name {
												if elapsed > int(time.Now().Unix()) {
													logrus.Infof("scheduling new chaos job for pod %s, step %s",
														podObject.Name, step.Name)
													nodeName := podObject.Spec.NodeName
													podName := podObject.Name
													step.Config["APP_POD"] = podName
													step.Config["CPU_CORES"] = "0"
													if step.Config["APP_CONTAINER"] == "" {
														step.Config["APP_CONTAINER"] = podObject.Spec.Containers[0].Name
													}
													step.Config["TOTAL_CHAOS_DURATION"] = fmt.Sprintf("%v", elapsed-int(time.Now().Unix()))
													step.Config["FILESYSTEM_UTILIZATION_PERCENTAGE"] = "0"
													step.Config["STRESS_TYPE"] = "pod-io-stress"
													job = podStressJob(chaosJobName, step, generation, nodeName, podName)
													_, err := chaos.KubeClient.BatchV1().Jobs(env.JobNamespace).Create(context.TODO(), &job, metaV1.CreateOptions{})
													if err != nil {
														// update status
														chaos.UpdateSnapshot(chaosJobName, step.Name, err.Error(), generation, v1alpha1.FailedStatus)
														chaos.UpdateJobStatus(chaosJobName, "", v1alpha1.FailedStatus)
														w.Stop()
														return
													}
													break
												}
											}
										}
										existed := false
										for _, p := range allPods {
											if p.Name == podObject.Name {
												existed = true
												break
											}
										}
										// if not in all pods
										if !existed {
											allPods = append(allPods, *podObject)
											expected := 0
											if percentage == 0 {
												expected = 1
											} else {
												expected = len(allPods) * percentage / 100
											}
											if expected > len(pods) && elapsed > int(time.Now().Unix()) {
												logrus.Infof("need to add a new job for the increment, scheduling new chaos job for pod %s, job %s",
													podObject.Name, step.Name)
												// need to scale up
												nodeName := podObject.Spec.NodeName
												podName := podObject.Name
												step.Config["APP_POD"] = podName
												step.Config["CPU_CORES"] = "0"
												if step.Config["APP_CONTAINER"] == "" {
													step.Config["APP_CONTAINER"] = podObject.Spec.Containers[0].Name
												}
												step.Config["TOTAL_CHAOS_DURATION"] = fmt.Sprintf("%v", elapsed-int(time.Now().Unix()))
												step.Config["FILESYSTEM_UTILIZATION_PERCENTAGE"] = "0"
												step.Config["STRESS_TYPE"] = "pod-io-stress"
												job = podStressJob(chaosJobName, step, generation, nodeName, podName)
												_, err := chaos.KubeClient.BatchV1().Jobs(env.JobNamespace).Create(context.TODO(), &job, metaV1.CreateOptions{})
												if err != nil {
													// update status
													chaos.UpdateSnapshot(chaosJobName, step.Name, err.Error(), generation, v1alpha1.FailedStatus)
													chaos.UpdateJobStatus(chaosJobName, "", v1alpha1.FailedStatus)
													w.Stop()
													return
												}
												pods = append(pods, *podObject)
											}
										}
									}
								}
							}
						}
					case watch.Deleted:
						// detect the deleted pod
						for i := range pods {
							if pods[i].Name == podObject.Name {
								logrus.Infof("new added pod %s detected, step %s",
									podObject.Name, step.Name)
								// remove from the pods list
								pods = append(pods[:i], pods[i+1:]...)
								break
							}
						}
						for i := range allPods {
							if allPods[i].Name == podObject.Name {
								logrus.Infof("remove %s from the allPods, step %s",
									podObject.Name, step.Name)
								// remove from the pods list
								allPods = append(allPods[:i], allPods[i+1:]...)
								break
							}
						}
					}
				}
			}
		}
	}()
	// todo maybe this is not very schoen
	time.Sleep(time.Duration(duration) * time.Second)
	// need cleanup here
	logrus.Infof("step %s finished, starting cleanup", step.Name)
	err := chaos.CleanJob(chaosJobName, step, generation)
	if err != nil {
		logrus.Errorf("step %s cleanup failed, reason: %s", step.Name, err.Error())
		// update status
		chaos.UpdateSnapshot(chaosJobName, step.Name, err.Error(), generation, v1alpha1.FailedStatus)
		chaos.UpdateJobStatus(chaosJobName, "", v1alpha1.FailedStatus)
		return
	}
	logrus.Infof("step %s cleanup done", step.Name)
	// set status to success
	chaos.UpdateSnapshot(chaosJobName, step.Name, "", generation, v1alpha1.SuccessStatus)
}
