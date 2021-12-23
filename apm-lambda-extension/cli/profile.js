'use strict'
const yaml = require('js-yaml')
const AWS = require('aws-sdk')
const fs = require('fs')
const { exec /* execFile */ } = require('child_process')
const { buildAndPublish } = require('./build-and-publish')

AWS.config.update({ region: 'us-west-2' })
const lambda = new AWS.Lambda({ apiVersion: '2015-03-31' })

function generateZipFile (pathSource, pathDest) {
  return new Promise(function (resolve, reject) {
    const env = Object.assign({}, process.env)
    exec(`rm -f ${pathDest} && cd ${pathSource} && zip -r ${pathDest} .`,
      env,
      function (error, stdout, stderr) {
        if (error) {
          reject(error)
        } else {
          resolve(stdout)
        }
      }
    )
  })
}

function convertStdoutTableToObject (string) {
  const split = string.split('┌')
  split.shift()
  const table = split.join('')
  const lines = table.replace(/[^\x00-\x7F]/g, '').split('\n').filter(item => item)
  let headers
  const results = []
  for (const [, line] of lines.entries()) {
    const cells = line.split(/\s{2,}/).filter((item) => item)
    if (!headers) {
      headers = cells
      continue
    }
    if (headers.length !== cells.length) {
      continue
    }
    const result = {}
    for (let i = 0; i < headers.length; i++) {
      result[headers[i]] = cells[i]
    }
    results.push(result)
  }
  return results
}

function createFunction (args) {
  return lambda.createFunction(args).promise()
}

async function cleanup (functionName) {
  await lambda.deleteFunction({
    FunctionName: functionName
  }).promise()
}

function generateTmpFunctionName (prefix) {
  const maxLengthLambda = 64
  const name = [prefix, 'apm-profile'].join('')
  if (name.length > maxLengthLambda) {
    console.log(`final function name ${name} is too long, bailing.`)
    process.exit(1)
  }
  return name
}

async function runScenario (scenario, config) {
  return new Promise(async function (resolve, reject) {
    const functionName = generateTmpFunctionName(scenario.function_name_prefix)
    const tmpZipName = `/tmp/${functionName}.zip`
    await generateZipFile(
      [__dirname, '/', scenario.code].join(''),
      tmpZipName
    )

    const createFunctionPromise = createFunction({
      FunctionName: functionName,
      Role: scenario.role,
      Code: {
        ZipFile: fs.readFileSync(tmpZipName)
      },
      Handler: scenario.handler,
      Runtime: scenario.runtime,
      Layers: scenario.layer_arns,
      Environment: {
        Variables: scenario.environment.variables
      }
    })

    if (!createFunctionPromise) {
      console.log('Could not call createFunction, bailing early')
      reject(new Error('Could not call createFunction, bailing early'))
    }
    createFunctionPromise.then(function (resultCreateFunction) {
      // need to wait for function to be created and its status
      // to no longer be PENDING before we throw traffic at it
      async function waitUntilNotPending (toRun, times = 0) {
        const maxTimes = 10
        const configuration = await lambda.getFunctionConfiguration({
          FunctionName: functionName
        }).promise()

        if (configuration.State === 'Pending' && times <= maxTimes) {
          console.log('waiting for function state != Pending')
          times++
          setTimeout(function () {
            waitUntilNotPending(toRun, times)
          }, 1000)
        } else if (times > maxTimes) {
          console.log('waited 10ish seconds and lambda did not activiate, bailing')
          process.exit(1)
        } else {
          toRun()
        }
      }
      waitUntilNotPending(function () {
        // invoke test runner here
        const times = config.config.n

        console.log(`Running profile command with -n ${times} (this may take a while)`)

        const env = Object.assign({}, process.env)
        exec(`${config.config.path_java} -jar ` +
             `${config.config.path_lpt_jar} -n ${times} ` +
             `-a ${resultCreateFunction.FunctionArn} `,
        env,
        function (error, stdout, stderr) {
          if (error) {
            reject(error)
            return
          }
          console.log('command done')
          console.log(stdout)
          cleanup(functionName)
          resolve((convertStdoutTableToObject(stdout)))
        }
        )
      })
    }).catch(function (e) {
      console.log('Error creating function')
      if (e.statusCode === 409 && e.code === 'ResourceConflictException') {
        console.log('Function already exists, deleting. Rerun profiler.')
        cleanup(functionName)
      } else {
        console.log(e)
      }
      reject(e)
    })
  })
}

async function runScenarios (config) {
  const all = []
  for (const [name, scenario] of Object.entries(config.scenarios)) {
    console.log(`starting ${name}`)
    try {
      all.push(await runScenario(scenario, config))
    } catch (e) {
      console.log('error calling runScenario')
    }
  }
  console.log(all)
}

const FLAG_LATEST = 'ELASTIC_LATEST'
function buildAndDeployArn (config) {
  return new Promise(function (resolve, reject) {
    const arns = Object.values(config.scenarios).map(function (item) {
      return item.layer_arns
    }).flat()

    if (arns.indexOf(FLAG_LATEST) === -1) {
      resolve()
    }

    // build latest arn, modify config to include it
    buildAndPublish().then(function result (arn) {
      for (const [key, item] of Object.entries(config.scenarios)) {
        for (let i = 0; i < item.layer_arns.length; i++) {
          if (config.scenarios[key].layer_arns[i] === FLAG_LATEST) {
            config.scenarios[key].layer_arns[i] = arn
          } else {
            console.log(config.scenarios[key].layer_arns[i])
          }
        }
      }
      resolve()
    })
  })
}

function cmd () {
  if (!fs.existsSync([__dirname, '/profile.yaml'].join(''))) {
    console.log('no profile.yaml found, please copy profile.yaml.dist and edit with your own values')
    return
  }
  const config = yaml.load(fs.readFileSync([__dirname, '/profile.yaml'].join(''))).profile
  // build and deploy the latest extension ARN (if neccesary)
  buildAndDeployArn(config).then(function () {
    runScenarios(config)
  })

  // return // tmp
  // run all our scenarios
}

module.exports = {
  cmd,

  // exported for testing
  convertStdoutTableToObject
}
