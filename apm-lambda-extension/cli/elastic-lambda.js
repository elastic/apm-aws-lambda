#!/usr/bin/env node
const yargs = require('yargs/yargs')
const {hideBin} = require('yargs/helpers')

if(!process.env.AWS_DEFAULT_REGION) {
  console.log('please set AWS_DEFAULT_REGION')
  process.exit(1)
}
const argv = yargs(hideBin(process.argv)).command(
  'update-layers [function_name] [layer_arn]',
  'updates or adds a layer ARN to a lambda\'s layers',
  function(yargs) {
  },
  async function(argv) {
    const {cmd} = require('./update-layers.js')
    cmd(argv)
  }
  )/*option('region', {
    alias: 'r',
    type: 'region',
    description: 'set aws region'
  })*/.demandCommand().recommendCommands().strict().parse()


