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

const fs = require('fs')
const yaml = require('js-yaml')
const { exec, /*execFile*/ } = require("child_process")
const shellescape = require('shell-escape');
function loadYaml() {
  return yaml.load(fs.readFileSync(__dirname + '/install.yaml'))
}

function runShellCommand(cmdString, args, text=''){
  return new Promise(function(resolve, reject){
    if(text) {
      console.log('# ' + text)
    }

    exec(cmdString + ' ' + shellescape(args), (error, stdout, stderr) => {

      if (error) {
          console.log(`error: ${error.message}`);
          console.log(`stderr: ${stderr}`);
          console.log(`stdout: ${stdout}`);
          // console.log(error)
          reject(error);
      }
      if(!error) {
        console.log(`stdout: ${stdout}`);
        return resolve(stdout);
      }
    });

  })
}

async function cmd(argv) {
  const config = loadYaml().install.config
  // console.log(install)
  // set some neccesary defaults
  config.lambda_env.ELASTIC_APM_CLOUD_PROVIDER = 'none'
  config.lambda_env.ELASTIC_APM_CENTRAL_CONFIG = 'false'

  // set extension's APM server env variable
  config.lambda_env.ELASTIC_APM_LAMBDA_APM_SERVER = config.lambda_env.ELASTIC_APM_SERVER_URL
  config.lambda_env.ELASTIC_APM_SERVER_URL = 'http://localhost:8200'


  // run command to set env variables
  try {
    const output = await runShellCommand(
      __dirname + '/elastic-lambda.js',
      ['update-function-env',config.function_name, JSON.stringify(config.lambda_env)],
      'updating lambda function\'s env variables'
    )
    console.log(output)
  } catch(error) {
    console.log('encountered error, exiting')
    process.exit(1)
  }

  // run command to build extension and publish layer
  let arn = false
  try {
    const output = await runShellCommand(
      __dirname + '/elastic-lambda.js',
      ['build-and-publish'],
      'building extension and publishing as layer'
    )
    const matches = output.match(/Published Layer as: (.*$)/m)
    if(matches) {
      arn = matches[1]
    }
    // console.log(output)
  } catch(error) {
    console.log('encountered error, exiting')
    process.exit(1)
  }


  // run command to update layer
  if(!arn) {
    console.log('could not extract arn from build-and-publish, exiting');
    process.exit(1)
  }
  try {
    const output = await runShellCommand(
      __dirname + '/elastic-lambda.js',
      ['update-layer', config.function_name, arn],
      'updating function layers configuration to use new layer'
    )
    // console.log(output)
  } catch(error) {
    console.log(error)
    console.log('encountered error, exiting')
    process.exit(1)
  }
}
module.exports = {
  cmd
}
