// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//	http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package plugins

import (
	"fmt"
	"runtime"

	"github.com/aws/aws-sdk-go/aws/request"
)

const (
	userAgentHeader = "User-Agent"
	// linuxBaseUserAgent is the base user agent string used for Linux.
	linuxBaseUserAgent = "aws-fluent-bit-plugin"
	// windowsBaseUserAgent is the base user agent string used for Windows.
	windowsBaseUserAgent = "aws-fluent-bit-plugin-windows"
)

// CustomUserAgentHandler returns a http request handler that sets a custom user agent to all aws requests
func CustomUserAgentHandler() request.NamedHandler {
	baseUserAgent := linuxBaseUserAgent
	if runtime.GOOS == "windows" {
		baseUserAgent = windowsBaseUserAgent
	}

	return request.NamedHandler{
		Name: "ECSLocalEndpointsAgentHandler",
		Fn: func(r *request.Request) {
			currentAgent := r.HTTPRequest.Header.Get(userAgentHeader)
			r.HTTPRequest.Header.Set(userAgentHeader,
				fmt.Sprintf("%s (%s) %s", baseUserAgent, runtime.GOOS, currentAgent))
		},
	}
}
