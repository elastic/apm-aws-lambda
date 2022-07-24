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

package logsapi_test

import (
	"elastic/apm-lambda-extension/logsapi"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	testCases := map[string]struct {
		opts        []logsapi.ClientOption
		expectedErr bool
	}{
		"empty": {
			expectedErr: true,
		},
		"missing base url": {
			opts: []logsapi.ClientOption{
				logsapi.WithLogsAPIBaseURL(""),
			},
			expectedErr: true,
		},
		"valid": {
			opts: []logsapi.ClientOption{
				logsapi.WithLogsAPIBaseURL("http://example.com"),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			_, err := logsapi.NewClient(tc.opts...)
			if tc.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSubscribe(t *testing.T) {
	testCases := map[string]struct {
		opts           []logsapi.ClientOption
		responseHeader int
		expectedErr    bool
	}{
		"valid response": {
			responseHeader: http.StatusOK,
			opts: []logsapi.ClientOption{
				logsapi.WithListenerAddress("localhost:0"),
			},
		},
		"invalid response": {
			responseHeader: http.StatusForbidden,
			opts: []logsapi.ClientOption{
				logsapi.WithListenerAddress("localhost:0"),
			},
			expectedErr: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var subRequest logsapi.SubscribeRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&subRequest))
				w.WriteHeader(tc.responseHeader)
			}))
			defer s.Close()

			c, err := logsapi.NewClient(append(tc.opts, logsapi.WithLogsAPIBaseURL(s.URL))...)
			require.NoError(t, err)

			if tc.expectedErr {
				require.Error(t, c.StartService([]logsapi.EventType{logsapi.Platform}, "foo"))
			} else {
				require.NoError(t, c.StartService([]logsapi.EventType{logsapi.Platform}, "foo"))
			}

			require.NoError(t, c.Shutdown())
		})
	}
}
