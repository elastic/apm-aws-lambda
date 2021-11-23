'use strict'
const yaml = require('js-yaml')
const AWS = require('aws-sdk')
const fs = require('fs')
const { exec, /*execFile*/ } = require("child_process")

AWS.config.update({region: 'us-west-2'});
const lambda = new AWS.Lambda({apiVersion:'2015-03-31'})

function convertStdoutTableToObject(string) {
  const split = string.split('┌')
  split.shift()
  const table = split.join('')
  const lines = table.replace(/[^\x00-\x7F]/g,'').split("\n").filter(item => item)
  let headers
  const results = [];
  for(const [,line] of lines.entries()) {
    const cells = line.split(/\s{2,}/).filter((item) => item)
    if(!headers) {
      headers = cells
      continue
    }
    if(headers.length !== cells.length) {
      continue;
    }
    const result = {}
    for(let i=0;i<headers.length;i++) {
      result[headers[i]] = cells[i]
    }
    results.push(result)
  }
  return results
}

function createFunction(args) {
  return lambda.createFunction(args).promise()
}

async function cleanup() {
  const deleteFunction = await lambda.deleteFunction({
    FunctionName: 'otel-automatic-test'
  }).promise();
}

async function cmd() {
  let createFunctionPromise = createFunction({
    FunctionName: 'otel-automatic-test',
    Role: 'arn:aws:iam::627286350134:role/service-role/http-api-node-role-egndxmyl',
    Code: {
      ZipFile: fs.readFileSync(__dirname + '/code.zip'),
    },
    Handler: 'index.handler',
    Runtime: 'nodejs14.x',
    Layers: [
      'arn:aws:lambda:us-west-2:627286350134:layer:otel-collector:1',
      'arn:aws:lambda:us-west-2:627286350134:layer:otel-nodejs:1'
    ],
    "Environment": {
      "Variables": {
        'AWS_LAMBDA_EXEC_WRAPPER':'/opt/otel-handler',
        'OPENTELEMETRY_COLLECTOR_CONFIG_FILE':'/var/task/collector.yaml',
        'OTEL_EXPORTER_OTLP_ENDPOINT':'http://localhost:55681/v1/traces',
        'OTEL_TRACES_SAMPLER':'AlwaysOn',
      }
    },
  })


  if(!createFunctionPromise) {
    console.log("Could not call createFunction, bailing early")
    return
  }
  createFunctionPromise.then(function(resultCreateFunction) {
    // need to wait for function to be created and its status
    // to no longer be PENDING before we throw traffic at it
    async function waitUntilNotPending(toRun, times=0) {
      const maxTimes = 10
      const configuration = await lambda.getFunctionConfiguration({
        FunctionName: 'otel-automatic-test'
      }).promise()

      if(configuration.State === "Pending" && times <= maxTimes) {
        console.log("waiting for function state != Pending");
        times++
        setTimeout(function(){
          waitUntilNotPending(toRun, times)
        }, 1000)
        return;
      } else if(times > maxTimes) {
        console.log("waited 10ish seconds and lambda did not activiate, bailing")
        process.exit(1)
      }
      else {
        toRun()
      }
    }
    waitUntilNotPending(function() {
      // invoke test runner here
      process.env['AWS_PROFILE'] = 'default'
      exec('/usr/local/opt/openjdk/bin/java -jar ' +
           __dirname + '/lpt-0.1.jar -n 10 ' +
           `-a ${resultCreateFunction.FunctionArn} ` +
           `-a ${resultCreateFunction.FunctionArn} `,
           {env:{AWS_PROFILE:'default', AWS_DEFAULT_REGION: 'us-west-2'}},
           function(error, stdout, stderr) {
             console.log("command done")
             console.log(stdout)
             console.log(convertStdoutTableToObject(stdout))
             cleanup()
           }
      )
    })
  }).catch(function(e){
    console.log('Error creating function')
    if(e.statusCode === 409 && e.code === 'ResourceConflictException') {
      console.log('Function already exists, deleting. Rerun profiler.')
      cleanup()
    }
  })



  // console.log(configuration.State)
  // console.log("deleting function")

  // cleanup()
}

module.exports = {
  cmd
}
