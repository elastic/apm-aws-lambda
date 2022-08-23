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

const { exec } = require('child_process')

/**
 * Cheap heuristic to get at last json output by command
 * @param {string} output
 */
function getLastJsonFromShellOutput (output) {
  const lines = output.split('\n')
  const jsonLines = []
  for (const line of lines) {
    if (line.trim() === '{' || jsonLines.length > 0) {
      jsonLines.push(line)
    }
  }

  const string = jsonLines.join('')
  const object = JSON.parse(string)
  return object
}

function buildAndPublish () {
  return new Promise(function (resolve, reject) {
    if (!process.env.ELASTIC_LAYER_NAME) {
      process.env.ELASTIC_LAYER_NAME = 'elastic-apm-extension'
    }
    console.log('running cd .. && make build-and-publish')
    exec('cd .. && make build-and-publish', (error, stdout, stderr) => {
      if (error) {
        console.log(`error: ${error.message}`)
        return
      }
      if (stderr) {
        console.log(`stderr: ${stderr}`)
        return
      }
      console.log(`stdout: ${stdout}`)
      const object = getLastJsonFromShellOutput(stdout)
      console.log(`Published Layer as: ${object.LayerVersionArn}`)
      resolve(object.LayerVersionArn)
    })
  })
}

function cmd () {
  buildAndPublish().then(function (arn) {
    console.log('FINAL: ' + arn)
  })
}

module.exports = {
  cmd,

  getLastJsonFromShellOutput,
  buildAndPublish
}
