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

package env

import (
	"github.com/sirupsen/logrus"
	"os"
	"strconv"
)

var (
	JobNamespace = populateEnv("JOB_NAMESPACE", "helm-operator-system").(string)
	LocalDebug   = populateEnv("LOCAL_DEBUG", false).(bool)
)

func populateEnv(name string, defaultValue any) any {
	if name == "LOCAL_DEBUG" {
		if os.Getenv(name) != "" {
			val, err := strconv.ParseBool(os.Getenv("LOCAL_DEBUG"))
			if err != nil {
				logrus.Fatalf("parse LOCAL_DEBUG error, reason, %s", err.Error())
			}
			return val
		}
	} else {
		if os.Getenv(name) != "" {
			return os.Getenv(name)
		}
	}

	return defaultValue
}

func ParseVars() {
	logrus.Info("vars for current run")
	logrus.Infof("LOCAL_DEBUG: %v", LocalDebug)
	logrus.Infof("JOB_NAMESPACE: %v", JobNamespace)
}
