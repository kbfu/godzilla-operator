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
	"errors"
	"fmt"
	"github.com/kbfu/godzilla-operator/api/v1alpha1"
	"github.com/kbfu/godzilla-operator/controllers/chaos/pod"
	"github.com/kbfu/godzilla-operator/controllers/env"
	"github.com/sirupsen/logrus"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func OverrideConfig(steps [][]v1alpha1.ChaosStep) {
	for i := range steps {
		for j := range steps[i] {
			config := pod.PopulateDefault(steps[i][j].Type)
			// override default config
			for k, v := range config.Env {
				_, ok := steps[i][j].Config[k]
				if !ok {
					steps[i][j].Config[k] = v
				}
			}

			if steps[i][j].Image == "" {
				steps[i][j].Image = config.Image
			}
			if steps[i][j].ServiceAccountName == "" {
				steps[i][j].ServiceAccountName = config.ServiceAccountName
			}
		}
	}
}

func PreCheck(job v1alpha1.GodzillaJob) error {
	dup := make(map[string]string)
	for _, parallelJobs := range job.Spec.Steps {
		for _, j := range parallelJobs {
			_, ok := dup[j.Name]
			if !ok {
				dup[j.Name] = ""
			} else {
				msg := fmt.Sprintf("duplicate step name found: %s", j.Name)
				err := UpdateJobStatus(fmt.Sprintf("%s-%v", job.Name, job.Generation),
					msg, v1alpha1.FailedStatus)
				if err != nil {
					logrus.Warn(err)
				}
				return errors.New(msg)
			}
			switch j.Type {
			case string(v1alpha1.LitmusPodDelete), string(v1alpha1.LitmusPodIoStress), string(v1alpha1.LitmusContainerKill),
				string(v1alpha1.LitmusPodCpuStress), string(v1alpha1.LitmusPodMemoryStress), string(v1alpha1.GodzillaPodNetworkDelay),
				string(v1alpha1.GodzillaPodNetworkCorruption), string(v1alpha1.GodzillaPodNetworkLoss), string(v1alpha1.GodzillaPodNetworkDuplicate),
				string(v1alpha1.GodzillaPodNetworkReorder):
			default:
				msg := fmt.Sprintf("unsupported type %s", j.Type)
				err := UpdateJobStatus(fmt.Sprintf("%s-%v", job.Name, job.Generation),
					msg, v1alpha1.FailedStatus)
				if err != nil {
					logrus.Warn(err)
				}
				return errors.New(msg)
			}
		}
	}
	return nil
}

func CleanJob(chaosJobName string, step v1alpha1.ChaosStep, generation int64) error {
	logrus.Infof("cleaning up the chaos job %s", step.Name)
	policy := metaV1.DeletePropagationForeground
	// get name
	jobList, err := env.KubeClient.BatchV1().Jobs(env.JobNamespace).List(context.TODO(), metaV1.ListOptions{
		LabelSelector: fmt.Sprintf("chaos.job.generation=%v,chaos.step.name=%s,chaos.job.name=%s", generation, step.Name, chaosJobName),
	})
	if err != nil {
		return err
	}
	for _, j := range jobList.Items {
		err := env.KubeClient.BatchV1().Jobs(env.JobNamespace).Delete(context.TODO(), j.Name, metaV1.DeleteOptions{
			PropagationPolicy: &policy,
		})
		if err != nil {
			return err
		}
	}
	return nil
}
