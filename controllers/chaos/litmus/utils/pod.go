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

package utils

import (
	"context"
	"github.com/kbfu/godzilla-operator/controllers/chaos"
	"github.com/sirupsen/logrus"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
	"strings"
)

func FilterTargetPods(config map[string]string) (pods []coreV1.Pod, err error) {
	var targetPods []string
	if config["TARGET_PODS"] != "" {
		targetPods = strings.Split(config["TARGET_PODS"], ",")
	}
	var percentage int
	if config["PODS_AFFECTED_PERC"] == "" {
		percentage = 0
	} else {
		percentage, _ = strconv.Atoi(config["PODS_AFFECTED_PERC"])
	}

	// if TARGET_PODS is not empty, use it
	if len(targetPods) > 0 {
		for _, targetPod := range targetPods {
			targetPod = strings.TrimSpace(targetPod)
			var podObject *coreV1.Pod
			podObject, err = chaos.KubeClient.CoreV1().Pods(config["APP_NAMESPACE"]).Get(context.TODO(), targetPod, metaV1.GetOptions{})
			if err != nil {
				return
			}
			pods = append(pods, *podObject)
		}
	} else {
		// check label
		var podList *coreV1.PodList
		podList, err = chaos.KubeClient.CoreV1().Pods(config["APP_NAMESPACE"]).List(context.TODO(), metaV1.ListOptions{
			LabelSelector: config["APP_LABEL"],
			FieldSelector: "status.phase=Running",
		})
		if err != nil {
			return
		}
		for i := range podList.Items {
			for _, condition := range podList.Items[i].Status.Conditions {
				if condition.Type == coreV1.ContainersReady && condition.Status == coreV1.ConditionTrue {
					if podList.Items[i].ObjectMeta.DeletionTimestamp == nil {
						pods = append(pods, podList.Items[i])
					}
				}
			}
		}
	}
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
	return
}
