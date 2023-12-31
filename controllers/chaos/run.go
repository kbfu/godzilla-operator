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
	"github.com/kbfu/godzilla-operator/api/v1alpha1"
	"github.com/sirupsen/logrus"
)

func Run(jobName string, step v1alpha1.ChaosStep, generation int64) {
	switch step.Type {
	case string(v1alpha1.LitmusPodDelete):
		runPodKill(jobName, step, generation)
	case string(v1alpha1.LitmusPodIoStress), string(v1alpha1.LitmusPodMemoryStress), string(v1alpha1.LitmusPodCpuStress):
		runPodStress(jobName, step, generation)
	case string(v1alpha1.LitmusContainerKill):
		runContainerKill(jobName, step, generation)
	case string(v1alpha1.GodzillaPodNetworkDelay), string(v1alpha1.GodzillaPodNetworkCorruption), string(v1alpha1.GodzillaPodNetworkLoss),
		string(v1alpha1.GodzillaPodNetworkDuplicate), string(v1alpha1.GodzillaPodNetworkReorder):
		runNetworkChaos(jobName, step, generation)
	case string(v1alpha1.GodzillaPodAutoscaler):
		runPodAutoscaler(jobName, step, generation)
	case string(v1alpha1.GodzillaPodDiskFill):
		runPodDiskFill(jobName, step, generation)
	default:
		logrus.Errorf("%s type not found", step.Type)
	}
}
