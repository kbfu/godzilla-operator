/*
 *
 *  * Licensed to the Apache Software Foundation (ASF) under one
 *  * or more contributor license agreements.  See the NOTICE file
 *  * distributed with this work for additional information
 *  * regarding copyright ownership.  The ASF licenses this file
 *  * to you under the Apache License, Version 2.0 (the
 *  * "License"); you may not use this file except in compliance
 *  * with the License.  You may obtain a copy of the License at
 *  *
 *  *     http://www.apache.org/licenses/LICENSE-2.0
 *  *
 *  * Unless required by applicable law or agreed to in writing, software
 *  * distributed under the License is distributed on an "AS IS" BASIS,
 *  * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  * See the License for the specific language governing permissions and
 *  * limitations under the License.
 *
 *
 */

package pod

import (
	_ "embed"
	godzillachaosiov1alpha1 "github.com/kbfu/godzilla-operator/api/v1alpha1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type PodConfig struct {
	Image              string            `yaml:"image"`
	ServiceAccountName string            `yaml:"serviceAccountName"`
	Env                map[string]string `yaml:"env"`
}

var (
	//go:embed common.yaml
	common []byte
	//go:embed pod-delete.yaml
	deletePod []byte
	//go:embed pod-io-stress.yaml
	podIoStress []byte
	//go:embed container-kill.yaml
	containerKill []byte
	//go:embed pod-memory-stress.yaml
	podMemoryStress []byte
	//go:embed pod-cpu-stress.yaml
	podCpuStress []byte
	//go:embed pod-network-delay.yaml
	podNetworkDelay []byte
)

func PopulateDefault(chaosType string) (config PodConfig) {
	var (
		commonConfig PodConfig
		typeConfig   PodConfig
	)
	err := yaml.Unmarshal(common, &commonConfig)
	if err != nil {
		logrus.Fatalf("unmarshal default pod common yaml file failed, reason: %s", err.Error())
	}
	switch chaosType {
	case string(godzillachaosiov1alpha1.LitmusPodDelete):
		err = yaml.Unmarshal(deletePod, &typeConfig)
		if err != nil {
			logrus.Fatalf("unmarshal pod delete yaml file failed, reason: %s", err.Error())
		}
	case string(godzillachaosiov1alpha1.LitmusPodIoStress):
		err = yaml.Unmarshal(podIoStress, &typeConfig)
		if err != nil {
			logrus.Fatalf("unmarshal pod io stress yaml file failed, reason: %s", err.Error())
		}
	case string(godzillachaosiov1alpha1.LitmusContainerKill):
		err = yaml.Unmarshal(containerKill, &typeConfig)
		if err != nil {
			logrus.Fatalf("unmarshal pod container kill yaml file failed, reason: %s", err.Error())
		}
	case string(godzillachaosiov1alpha1.LitmusPodMemoryStress):
		err = yaml.Unmarshal(podMemoryStress, &typeConfig)
		if err != nil {
			logrus.Fatalf("unmarshal pod memory stress yaml file failed, reason: %s", err.Error())
		}
	case string(godzillachaosiov1alpha1.LitmusPodCpuStress):
		err = yaml.Unmarshal(podCpuStress, &typeConfig)
		if err != nil {
			logrus.Fatalf("unmarshal pod cpu stress yaml file failed, reason: %s", err.Error())
		}
	case string(godzillachaosiov1alpha1.GodzillaPodNetworkDelay):
		err = yaml.Unmarshal(podNetworkDelay, &typeConfig)
		if err != nil {
			logrus.Fatalf("unmarshal pod network delay yaml file failed, reason: %s", err.Error())
		}
	}
	config = commonConfig
	if config.Env == nil {
		config.Env = make(map[string]string)
	}
	if typeConfig.Image != "" {
		config.Image = typeConfig.Image
	}
	for k, v := range typeConfig.Env {
		config.Env[k] = v
	}
	if typeConfig.ServiceAccountName != "" {
		config.ServiceAccountName = typeConfig.ServiceAccountName
	}

	return config
}
