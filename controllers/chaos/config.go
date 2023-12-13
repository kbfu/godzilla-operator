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

package chaos

import (
	"github.com/kbfu/godzilla-operator/controllers/env"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	KubeClient *kubernetes.Clientset
	config     *rest.Config
	Client     client.Client
)

func InitKubeClient(scheme *runtime.Scheme) {
	var err error
	if env.LocalDebug {
		config, err = clientcmd.BuildConfigFromFlags("", homedir.HomeDir()+"/.kube/config")
		if err != nil {
			logrus.Fatalf("get config error, reason: %s", err.Error())
		}
		KubeClient, err = kubernetes.NewForConfig(config)
		if err != nil {
			logrus.Fatalf("get client set error, reason: %s", err.Error())
		}
		Client, err = client.New(config, client.Options{
			Scheme: scheme,
		})
		if err != nil {
			logrus.Fatalf("get client error, reason: %s", err.Error())
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			logrus.Fatalf("get in cluster config error, reason: %s", err.Error())
		}
		KubeClient, err = kubernetes.NewForConfig(config)
		if err != nil {
			logrus.Fatalf("get in cluster client set error, reason: %s", err.Error())
		}
		Client, err = client.New(config, client.Options{
			Scheme: scheme,
		})
		if err != nil {
			logrus.Fatalf("get client error, reason: %s", err.Error())
		}
	}
}
