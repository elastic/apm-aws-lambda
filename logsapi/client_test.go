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
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/elastic/apm-aws-lambda/logsapi"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
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
				logsapi.WithLogger(zaptest.NewLogger(t).Sugar()),
			},
			expectedErr: true,
		},
		"missing logger": {
			opts: []logsapi.ClientOption{
				logsapi.WithLogsAPIBaseURL("http://example.com"),
			},
			expectedErr: true,
		},
		"valid": {
			opts: []logsapi.ClientOption{
				logsapi.WithLogsAPIBaseURL("http://example.com"),
				logsapi.WithLogger(zaptest.NewLogger(t).Sugar()),
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
				logsapi.WithLogger(zaptest.NewLogger(t).Sugar()),
			},
		},
		"invalid response": {
			responseHeader: http.StatusForbidden,
			opts: []logsapi.ClientOption{
				logsapi.WithListenerAddress("localhost:0"),
				logsapi.WithLogger(zaptest.NewLogger(t).Sugar()),
			},
			expectedErr: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var subRequest logsapi.SubscribeRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&subRequest))
				_, err := url.ParseRequestURI(subRequest.Destination.URI)
				require.NoError(t, err)
				w.WriteHeader(tc.responseHeader)
			}))
			defer s.Close()

			cOpts := append(tc.opts, logsapi.WithLogsAPIBaseURL(s.URL), logsapi.WithLogsAPISubscriptionTypes(logsapi.Platform))
			c, err := logsapi.NewClient(cOpts...)
			require.NoError(t, err)

			if tc.expectedErr {
				require.Error(t, c.StartService("foo"))
			} else {
				require.NoError(t, c.StartService("foo"))
			}

			require.NoError(t, c.Shutdown())
		})
	}
}

func TestSubscribeAWSRequest(t *testing.T) {
	addr := "localhost:8080"

	testCases := map[string]struct {
		opts []logsapi.ClientOption
	}{
		"valid response": {
			opts: []logsapi.ClientOption{
				logsapi.WithListenerAddress(addr),
				logsapi.WithLogger(zaptest.NewLogger(t).Sugar()),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var subRequest logsapi.SubscribeRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&subRequest))
				_, err := url.ParseRequestURI(subRequest.Destination.URI)
				require.NoError(t, err)
				w.WriteHeader(http.StatusOK)
			}))
			defer s.Close()

			cOpts := append(
				tc.opts,
				logsapi.WithLogsAPIBaseURL(s.URL),
				logsapi.WithLogBuffer(1),
				logsapi.WithLogsAPISubscriptionTypes(logsapi.Platform, logsapi.Function),
			)
			c, err := logsapi.NewClient(cOpts...)
			require.NoError(t, err)
			require.NoError(t, c.StartService("testID"))

			// Create a request to send to the logs listener
			platformDoneEvent := `{
		"time": "2021-02-04T20:00:05.123Z",
		"type": "platform.runtimeDone",
		"record": {
		   "requestId":"6f7f0961f83442118a7af6fe80b88",
		   "status": "success"
		}
	}`
			body := []byte(`[` + platformDoneEvent + `]`)
			req, err := http.NewRequest(http.MethodGet, "http://"+addr, bytes.NewReader(body))
			require.NoError(t, err)

			// Send the request to the logs listener
			rsp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rsp.StatusCode)
			require.NoError(t, rsp.Body.Close())
			require.NoError(t, c.Shutdown())
		})
	}
}
