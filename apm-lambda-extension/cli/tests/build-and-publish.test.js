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

const tap = require('tap')

const { getLastJsonFromShellOutput } = require('../build-and-publish')

tap.test('all', function (t) {
  const fixture = `... AWS_SECRET_ACCESS_KEY=h...E ELASTIC_LAYER_NAME=apm-lambda-extension make build-and-publish
  make build
  GOOS=linux GOARCH=amd64 go build -o bin/extensions/apm-lambda-extension main.go
  cd bin && rm -f extension.zip || true && zip -r extension.zip extensions
    adding: extensions/ (stored 0%)
    adding: extensions/apm-lambda-extension (deflated 47%)
  aws lambda publish-layer-version --layer-name "apm-lambda-extension" --zip-file "fileb://./bin/extension.zip"
  {
      "Content": {
          "Location": "https://awslambda-us-west-2-layers.s3.us-west-2.amazonaws.com/snapshots/627286350134/apm-lambda-extension-133cc08d-7fa4-455a-b5f5-2c648340fd62?versionId=_BL4IK30YdEwsVXR9uy0F5Y3b45T8_cc&X-Amz-Security-Token=IQoJb3JpZ2luX2VjEC0aCXVzLXdlc3QtMiJHMEUCIQCHpeYGX6XMXHWSot8KJBrfIggLEz3vQy4dmza2z2ntFQIgCkMm3r5Fb7UDFDPjovY9UdNhaFo0RiH3T8sWK35Hys4qgwQIlv%2F%2F%2F%2F%2F%2F%2F%2F%2F%2FARADGgw1MDIyOTcwNzYxNjMiDJBxS6Jy9he2yLPUKSrXA8Pv%2BLDpIF8Xr5kcR3cXsAM4TAM9ebYvEBVxi%2BbDfrMx0EJ6JWxG8RDpapFsGylqrlvLSCVXwddfpP1tlgR2USTpKnOMPRPTQuFUfXYrE%2BVyzC617p81QfH1I7zQ%2FoDSmi1rb%2BrK6YvD5eLHYWNd%2FWbW1dlgxkBPkIvy0HEvdQH8tPkFGgGDvCVOEttTO4hAPKvvAja%2FuAWM%2Bi9S%2BH8WAhhtlBwUsRL8f7mK57Et0%2B5szkVBvELQlRPCgG5LHLC9FeUoLtWV49yCHXgwYvpjlSnwFzBxBu4eSKKOuar4%2FKt1N3OCIzelDuRo0OOFhqhBr1Ptg6uGmOaamf3tu%2BVLJz64X1OxcYIhZ0%2BszN1B5tFnX6XQOM7P435Y7%2FHCiEMysYXvMTdTj5P%2BwOZBmeDnPjhrexteqwR6fqD58x9iJrPv1sJFdGCC1SfJ9N%2FHlCW6FHmKqJU9DPA7yUz8dxUgb8tXNp37SALj%2FEpT4%2F3webOl0rjx6tIJH9sdLfKSS92p4qg%2BCUDPHfogNgKaeZppoF2VzSJnYoY%2Fz1n4ZMxWHFvbpxQAqLra4pSgO8B6m%2F7AvAf3yeHaZ5UOUpjqoqihCKDLD1ad%2Bbl7HV6tyg970OEGxuhUcBbkZDCp08iKBjqlAec5KVUKhsaSTYJ38YymI%2FotBQOoRhr45F1gxnPD6cj%2FHiuXTdecRUp10GXMpyThzBKJCFAAEl4XQpSC%2BFzC0F%2ByTw1FyPyYwUZtJT%2BHLNZwqCFdctHC3vMPGX9%2By1ErO7iuUFtvu%2Bz0KYmXO4hs8KA3dmL3eiiSDkBAA6%2BxUINmrnbeyb0tK37eo92YEx8qxgWi0tnLvLhTyNuujR0CPq%2BQ6dTgaA%3D%3D&X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Date=20210927T203727Z&X-Amz-SignedHeaders=host&X-Amz-Expires=600&X-Amz-Credential=ASIAXJ4Z5EHB535UQHE4%2F20210927%2Fus-west-2%2Fs3%2Faws4_request&X-Amz-Signature=c77a686b839a045fe2b528bc6aa084ba6f05bf1d49b54dfc0e7e9429cefee964",
          "CodeSha256": "5...=",
          "CodeSize": 33
      },
      "LayerArn": "arn...extension",
      "LayerVersionArn": "arn...extension:13",
      "Description": "",
      "CreatedDate": "2021-09-27T20:37:28.446+0000",
      "Version": 99
  }`
  const object = getLastJsonFromShellOutput(fixture)
  t.equal(99, object.Version)
  t.end()
})
