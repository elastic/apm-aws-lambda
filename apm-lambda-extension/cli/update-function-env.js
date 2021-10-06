const AWS = require('aws-sdk')
AWS.config.update({region: process.env.AWS_DEFAULT_REGION});
async function cmd(argv) {
  const lambda = new AWS.Lambda({apiVersion:'2015-03-31'})
  const newEnv = JSON.parse(argv.env_as_json)
  if(!newEnv) {
    console.log("could not parse as json")
    console.log(argv.env_as_json)
    return
  }

  const configuration = await lambda.getFunction({
    FunctionName: argv.function_name
  }).promise()

  let env = {}
  if(configuration.Configuration.Environment) {
    env = configuration.Configuration.Environment.Variables
  }

  for(const [key,value] of Object.entries(newEnv)) {
    env[key] = value
  }

  const result = await lambda.updateFunctionConfiguration({
    FunctionName: argv.function_name,
    Environment:{
      Variables:env
    }
  }).promise()
  console.log(result)
}

module.exports = {
  cmd
}
