/*
 * Copyright 2022 CloudWeGo Authors
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
 */

package utils

import "net"

const (
	UNKNOWN_IP_ADDR = "-"
)

var localIP string

// LocalIP returns host's ip
func LocalIP() string {
	return localIP
}

// getLocalIp enumerates local net interfaces to find local ip, it should only be called in init phase
func getLocalIp() string {
	inters, err := net.Interfaces()
	if err != nil {
		return UNKNOWN_IP_ADDR
	}
	for _, inter := range inters {
		if inter.Flags&net.FlagLoopback != net.FlagLoopback &&
			inter.Flags&net.FlagUp != 0 {
			addrs, err := inter.Addrs()
			if err != nil {
				return UNKNOWN_IP_ADDR
			}
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					return ipnet.IP.String()
				}
			}
		}
	}

	return UNKNOWN_IP_ADDR
}

func init() {
	localIP = getLocalIp()
}

// TLSRecordHeaderLooksLikeHTTP reports whether a TLS record header
// looks like it might've been a misdirected plaintext HTTP request.
func TLSRecordHeaderLooksLikeHTTP(hdr [5]byte) bool {
	switch string(hdr[:]) {
	case "GET /", "HEAD ", "POST ", "PUT /", "OPTIO":
		return true
	}
	return false
}
