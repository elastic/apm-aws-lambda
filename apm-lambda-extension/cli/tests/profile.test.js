// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

const tap = require('tap');
const {convertStdoutTableToObject} = require('../profile')

tap.test('all', function(t) {
  const emptyResult = convertStdoutTableToObject('')
  t.same(emptyResult.length, 0, 'no values returned for invalid table')

  const resultsActual = convertStdoutTableToObject(`some stuff
  in front to make sure it's stripped.

  ┌────────────────────────────┬────────────────────────────┬────────────────────────────┬───────────────────────────┬───────────────────────────┬───────────────────────────┬───────────────────────────┐
  │       Function name        │         Throughput         │          Avg. RT           │          Min. RT          │            p95            │            p99            │          Max. RT          │
  ├────────────────────────────┼────────────────────────────┼────────────────────────────┼───────────────────────────┼───────────────────────────┼───────────────────────────┼───────────────────────────┤
  │    otel-automatic-test     │           735.3            │             81             │            60             │            100            │            100            │            100            │
  │    otel-manual-test        │           734.3            │             82             │            61             │            101            │            105            │            110            │
  └────────────────────────────┴────────────────────────────┴────────────────────────────┴───────────────────────────┴───────────────────────────┴───────────────────────────┴───────────────────────────┘
  same for after
  `)
  t.same(resultsActual.length, 2, 'two rows')
  t.same(resultsActual[0]['Function name'],'otel-automatic-test','column one read')
  t.same(resultsActual[0]['Throughput'],'735.3','column two read')
  t.same(resultsActual[0]['Avg. RT'],'81','column three read')
  t.same(resultsActual[0]['Min. RT'],'60','column four read')
  t.same(resultsActual[0]['Max. RT'],'100','column five read')
  t.same(resultsActual[0]['p95'],'100','column six read')
  t.same(resultsActual[0]['p99'],'100','column seven read')

  t.same(resultsActual[1]['Function name'],'otel-manual-test','column one read')
  t.same(resultsActual[1]['Throughput'],'734.3','column two read')
  t.same(resultsActual[1]['Avg. RT'],'82','column three read')
  t.same(resultsActual[1]['Min. RT'],'61','column four read')
  t.same(resultsActual[1]['Max. RT'],'110','column five read')
  t.same(resultsActual[1]['p95'],'101','column six read')
  t.same(resultsActual[1]['p99'],'105','column seven read')

  t.end()
})
