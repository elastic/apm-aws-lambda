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

const AWS = require('aws-sdk')
AWS.config.update({ region: process.env.AWS_DEFAULT_REGION })
async function cmd (argv) {
  const lambda = new AWS.Lambda({ apiVersion: '2015-03-31' })
  const newEnv = JSON.parse(argv.env_as_json)
  if (!newEnv) {
    console.log('could not parse as json')
    console.log(argv.env_as_json)
    return
  }

  const configuration = await lambda.getFunction({
    FunctionName: argv.function_name
  }).promise()

  let env = {}
  if (configuration.Configuration.Environment) {
    env = configuration.Configuration.Environment.Variables
  }

  for (const [key, value] of Object.entries(newEnv)) {
    env[key] = value
  }

  const result = await lambda.updateFunctionConfiguration({
    FunctionName: argv.function_name,
    Environment: {
      Variables: env
    }
  }).promise()
  console.log(result)
}

module.exports = {
  cmd
}
