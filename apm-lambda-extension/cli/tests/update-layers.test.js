const tap = require('tap');
const {getNewLayersArray} = require('../update-layers')

tap.test('all', function(t){
  const fixtures = [
    {
      arn:'arn:aws:lambda:us-west-2:xxxxxxxxxxxx:layer:apm-lambda-extension:12',
      layers:[{Arn:'foo'},{Arn:'bar'}],
      expected:['foo','bar','arn:aws:lambda:us-west-2:xxxxxxxxxxxx:layer:apm-lambda-extension:12']
    },
    {
      arn:'arn:aws:lambda:us-west-2:xxxxxxxxxxxx:layer:apm-lambda-extension:9',
      layers:[{Arn:'foo'},{Arn:'bar'},{Arn:'arn:aws:lambda:us-west-2:xxxxxxxxxxxx:layer:apm-lambda-extension:12'}],
      expected:['foo','bar','arn:aws:lambda:us-west-2:xxxxxxxxxxxx:layer:apm-lambda-extension:9']
    },
  ]
  for(const i in fixtures) {
    const fixture = fixtures[i]
    const result = getNewLayersArray(
      fixture.arn,
      fixture.layers
    )
    t.ok(Array.isArray(result))
    t.same(result, fixture.expected)
  }
  t.end()
});
