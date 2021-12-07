const apm = require('elastic-apm-node').start({})
exports.handler = apm.lambda(function handler (event, context, callback) {
  return new Promise(function (resolve, reject) {
    const response = {
      statusCode: 200,
      body: 'hello simple client!'
    }
    resolve(response)
  })
})
