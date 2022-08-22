// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package apmproxy

// Constants for the state of the transport used in
// the backoff implementation.
type Status string

const (
	// The apmproxy started but no information can be
	// inferred on the status of the transport.
	// Either because the apmproxy just started and no
	// request was forwarded or because it recovered
	// from a failure.
	Started Status = "Started"

	// Last request completed successfully.
	Healthy Status = "Healthy"

	// Last request failed.
	Failing Status = "Failing"

	// The APM Server returned status 429 and the extension
	// was ratelimited.
	RateLimited Status = "RateLimited"
)
