const AWS = require('aws-sdk')
AWS.config.update({region: process.env.AWS_DEFAULT_REGION});

function getUnversionedArn(arn) {
  const parts = arn.split(':')
  parts.pop()
  const unversionedArn = parts.join(':')
  return unversionedArn
}

/**
 * Returns an array of layer names, suitable for updateFunctionConfiguration
 *
 * The function will search through layers and attempt to match the unversioned
 * form of arn.  If found, the function will _replace_ the arn in layers with
 * the new arn.  If not found, arn will be added to the layers.
 *
 * @param {string} arn The new layer arn to add or update
 * @param {array} layers An array of the currently configured layers from getFunction
 */
function getNewLayersArray(arn, layers) {
  const layerNames = layers.map(function(item){
    return item.Arn
  })

  const unversionedArn = getUnversionedArn(arn)

  let foundArn = false
  for(const i in layerNames) {
    const name = layerNames[i]
    if(unversionedArn === getUnversionedArn(name)) {
      layerNames[i] = arn
      foundArn = true
    }
  }
  if(!foundArn) {
    layerNames.push(arn)
  }
  return layerNames
}

async function cmd(argv) {
  const lambda = new AWS.Lambda({apiVersion:'2015-03-31'})
  const configuration = await lambda.getFunction({
    FunctionName: argv.function_name
  }).promise();

  const layers = getNewLayersArray(argv.layer_arn, configuration.Configuration.Layers)

  const results = await lambda.updateFunctionConfiguration({
    FunctionName: argv.function_name,
    Layers: layers
  }).promise()
  // console.log(configuration.Configuration.Layers);
  console.log(results)
}

module.exports = {
  cmd,
  getNewLayersArray
}
