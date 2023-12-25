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
	"net"
)

func LookUpHost(hostname string) (addrs []string, err error) {
	ips, err := net.LookupHost(hostname)
	if err != nil {
		return
	}
	for _, ip := range ips {
		if net.ParseIP(ip).To4() != nil {
			addrs = append(addrs, ip)
		}
	}
	return
}
