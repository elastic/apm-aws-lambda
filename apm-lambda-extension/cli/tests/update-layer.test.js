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
const {getNewLayersArray} = require('../update-layer')

tap.test('all', function(t){
  const fixtures = [
    {
      arn:'arn:aws:lambda:us-west-2:xxxxxxxxxxxx:layer:apm-lambda-extension:12',
      layers:[{Arn:'foo'},{Arn:'bar'}],
      expected:['foo','bar','arn:aws:lambda:us-west-2:xxxxxxxxxxxx:layer:apm-lambda-extension:12']
    },
    {
      arn:'arn:aws:lambda:us-west-2:xxxxxxxxxxxx:layer:apm-lambda-extension:9',
      layers:[{Arn:'foo'},{Arn:'bar'},{Arn:'arn:aws:lambda:us-west-2:xxxxxxxxxxxx:layer:apm-lambda-extension:12'}],
      expected:['foo','bar','arn:aws:lambda:us-west-2:xxxxxxxxxxxx:layer:apm-lambda-extension:9']
    },
  ]
  for(const i in fixtures) {
    const fixture = fixtures[i]
    const result = getNewLayersArray(
      fixture.arn,
      fixture.layers
    )
    t.ok(Array.isArray(result))
    t.same(result, fixture.expected)
  }
  t.end()
});
