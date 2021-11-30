'use strict'
const yaml = require('js-yaml')
const AWS = require('aws-sdk')
const fs = require('fs')
const { exec, /*execFile*/ } = require("child_process")
const { exit } = require('process')

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

async function cleanup(functionName) {
  const deleteFunction = await lambda.deleteFunction({
    FunctionName: functionName
  }).promise();
}

function generateTmpFunctionName(prefix) {
  const maxLengthLambda = 64
  const name = [prefix,'apm-profile'].join('')
  if(name.length > maxLengthLambda) {
    console.log(`final function name ${name} is too long, bailing.`);
    process.exit(1)
  }
  return name
}

async function cmd() {
  const config = yaml.load(fs.readFileSync(__dirname + '/profile.yaml')).profile
  const scenario = config.scenarios.otel
  const functionName = generateTmpFunctionName(scenario.function_name_prefix)

  let createFunctionPromise = createFunction({
    FunctionName: functionName,
    Role: scenario.role,
    Code: {
      ZipFile: fs.readFileSync(__dirname + '/' + scenario.code),
    },
    Handler: scenario.handler,
    Runtime: scenario.runtime,
    Layers: scenario.layer_arns,
    "Environment": {
      "Variables": scenario.environment.variables
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
        FunctionName: functionName
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
      const times = config.config.n

      console.log(`Running profile command with -n ${times} (this may take a while)`);

      const env = Object.assign({}, process.env)
      exec(`${config.config.path_java} -jar ` +
           `${config.config.path_lpt_jar} -n ${times} ` +
           `-a ${resultCreateFunction.FunctionArn} `,
           env,
           function(error, stdout, stderr) {
             console.log("command done")
             console.log(stdout)
             console.log(convertStdoutTableToObject(stdout))
             cleanup(functionName)
           }
      )
    })
  }).catch(function(e){
    console.log('Error creating function')
    if(e.statusCode === 409 && e.code === 'ResourceConflictException') {
      console.log('Function already exists, deleting. Rerun profiler.')
      cleanup(functionName)
    } else {
      console.log(e)
    }
  })



  // console.log(configuration.State)
  // console.log("deleting function")

  // cleanup()
}

module.exports = {
  cmd,

  // exported for testing
  convertStdoutTableToObject
}
