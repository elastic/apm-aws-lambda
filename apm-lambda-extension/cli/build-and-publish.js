const { ProcessCredentials } = require("aws-sdk")
const { exec } = require("child_process")

/**
 * Cheap heuristic to get at last json output by command
 * @param {string} output
 */
function getLastJsonFromShellOutput(output) {
  const lines = output.split("\n")
  const jsonLines = []
  for(const line of lines) {
    if(line.trim() === '{' || jsonLines.length > 0) {
      jsonLines.push(line)
    }
  }
  const string = jsonLines.join('')
  const object = JSON.parse(string)
  return object
}

function cmd() {
  if(!process.env['ELASTIC_LAYER_NAME']) {
    process.env['ELASTIC_LAYER_NAME'] = 'apm-lambda-extension'
  }
  console.log(`running cd .. && make build-and-publish`)
  exec("cd .. && make build-and-publish", (error, stdout, stderr) => {
    if (error) {
        console.log(`error: ${error.message}`);
        return;
    }
    if (stderr) {
        console.log(`stderr: ${stderr}`);
        return;
    }
    console.log(`stdout: ${stdout}`);
    const object = getLastJsonFromShellOutput(stdout)
    console.log(`Published Layer as: ${object.LayerVersionArn}`)
  });
}

module.exports = {
  cmd,

  getLastJsonFromShellOutput
}
