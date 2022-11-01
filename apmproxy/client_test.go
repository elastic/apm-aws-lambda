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

package apmproxy_test

import (
	"testing"

	"github.com/elastic/apm-aws-lambda/apmproxy"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestClient(t *testing.T) {
	testCases := map[string]struct {
		opts        []apmproxy.Option
		expectedErr bool
	}{
		"empty": {
			expectedErr: true,
		},
		"missing base url": {
			opts:        []apmproxy.Option{},
			expectedErr: true,
		},
		"missing logger": {
			opts: []apmproxy.Option{
				apmproxy.WithURL("https://example.com"),
			},
			expectedErr: true,
		},
		"valid": {
			opts: []apmproxy.Option{
				apmproxy.WithURL("https://example.com"),
				apmproxy.WithLogger(zaptest.NewLogger(t).Sugar()),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			_, err := apmproxy.NewClient(tc.opts...)
			if tc.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
