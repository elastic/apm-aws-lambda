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
  'update-layer [function_name] [layer_arn]',
  "updates or adds a layer ARN to a lambda's layers\n",
  function(yargs) {
  },
  async function(argv) {
    const {cmd} = require('./update-layer.js')
    cmd(argv)
  }
  ).command(
    'build-and-publish',
    "runs build-and-publish make command in ..\n",
    function(yargs) {
    },
    function(argv) {
      const {cmd} = require('./build-and-publish')
      cmd(argv)
    }
  ).command(
    'update-function-env [function_name] [env_as_json]',
    "adds env configuration to named lambda function\n",
    function(yargs) {
    },
    function(argv){
      checkAwsRegion()
      const {cmd} = require('./update-function-env')
      cmd(argv)
    }
  ).command(
    'install',
    'reads in install.yaml and run commands needed to install lambda product',
    function(yargs) {
    },
    function(argv) {
      const {cmd} = require('./install')
      cmd(argv)
    }
  ).demandCommand().recommendCommands().strict().parse()


