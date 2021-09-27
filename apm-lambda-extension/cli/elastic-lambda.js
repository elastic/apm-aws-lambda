#!/usr/bin/env node
const AWS = require('aws-sdk')
const yargs = require('yargs/yargs')
const {hideBin} = require('yargs/helpers')

function checkAwsRegion() {
  if(!process.env.AWS_DEFAULT_REGION) {
    console.log('please set AWS_DEFAULT_REGION')
    process.exit(1)
  }
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
  ).command(
    'build-and-publish',
    'runs build-and-publish make command in ..',
    function(yargs) {
    },
    function(argv) {
      const {cmd} = require('./build-and-publish')
      cmd(argv)
    }
  ).command(
    'update-function-env [function_name] [env_as_json]',
    'adds env configuration to named lambda function',
    function(yargs) {
    },
    function(argv){
      checkAwsRegion()
      const {cmd} = require('./update-function-env')
      cmd(argv)
    }
  ).demandCommand().recommendCommands().strict().parse()


