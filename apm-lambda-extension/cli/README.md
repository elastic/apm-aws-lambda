# Elastic Lambda APM CLI

This folder contains a Node.js based CLI application that users can use to install or update aspects of the Elastic Lambda APM product.  This is currently an experimental CLI. 

To use this command line application, you'll need 

- A version of Node.js and NPM installed your system (to run the command itself)
- A version of go installed on your system (to build the Lambda extension)
- A version of the aws cli tool (to publish your layer)

## Using the CLI

To install the application, use NPM install its dependencies

```
    % npm install
```
    
After running `npm install`, you can see a list of available commands by running     

    % ./elastic-lambda.js 
    elastic-lambda.js <command>

    Commands:
      elastic-lambda.js update-layer            updates or adds a layer ARN to a
      [function_name] [layer_arn]               lambda's layers

      elastic-lambda.js build-and-publish       runs build-and-publish make command
                                                in ..

      elastic-lambda.js update-function-env     adds env configuration to named
      [function_name] [env_as_json]             lambda function

      elastic-lambda.js install                 reads in install.yaml and run
                                                commands needed to install lambda
                                                product

    Options:
      --help     Show help                                                 [boolean]
      --version  Show version number                                       [boolean]

All of these commands require your have an `AWS_DEFAULT_REGION` environmental variable set, as well as environmental variable for authenticating against AWS web services (ex. `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`)

To use the all-in-one `install` command, you'll need to copy the `install.yaml.dist` file 

    $ cp install.yaml.dist install.yaml
    
and then edit the new `install.yaml` file such that it contains your function name, and your APM Agent configuration values. 

    install:
      config:
        layer_name: "apm-lambda-extension"
        function_name: "your-lambda-function"
        env:
          ELASTIC_APM_LOG_LEVEL: "info"
          ELASTIC_APM_SECRET_TOKEN: "..."
          ELASTIC_APM_SERVER_URL: "http://elastic.example.com:8200"
          ELASTIC_APM_SERVICE_NAME: "service name here"
          
Once you're done editing this file you'll run the `install` sub-command

    $ ./elastic-lambda.js install          
    //...
    
The `install` sub-command will automatically

1. Update your Lambda environmental variables     
2. Build the Lambda Extension Layer and Publish it to AWS
3. Add the just published layer to your Lambda function's configuration

## Running the Profiler

You can use the `./elastic-lambda.js profile` command to run performance _scenarios_ using the `lpt-0.1.jar` perf. runner. The `profile` sub-command expects a `profile.yaml` file to be present -- copy `profile.yaml.dist` as a starter file.  Thie configuration file contains the location of your downloaded `ltp-0.1.jar` file, and configuration for individual scenarios.

A scenario configuration looks like the following

      scenarios:
        # each section under scenarios represents a single lambda function
        # to deploy and test via lpt-0.1.jar
        otel:
          function_name_prefix: 'otel-autotest-'
          role: '[... enter role ...]'
          code: './profile/code'
          handler: 'index.handler'
          runtime: 'nodejs14.x'
          # up to five
          layer_arns:
            - '... enter first layer'
            - '... enter second layer'
            # use this value to trigger a build and deploy of the latest extension
            # - 'ELASTIC_LATEST'
          environment:
            variables:
              AWS_LAMBDA_EXEC_WRAPPER: '/opt/otel-handler'
              OPENTELEMETRY_COLLECTOR_CONFIG_FILE: '/var/task/collector.yaml'
              OTEL_EXPORTER_OTLP_ENDPOINT: 'http://localhost:55681/v1/traces'
              OTEL_TRACES_SAMPLER: 'AlwaysOn'
              APM_ELASTIC_SECRET_TOKEN: '[... enter secret token ...]'
              ELASTIC_APM_SERVER_URL: '[... enter APM Server URL ...]'
              
Each individual object under the `scenarios` key represents an individual perf. scenario.

**`function_name_prefix`**              

The `profile` sub-command will use the `function_name_prefix` configuration value when naming the Lambda function it creates and deploys.  This helps ensure your function name will be complete. 

**`role`**

AWS needs a _role_ in order to create a Lambda function.  Use the `role` field to provide this value.

**`code`**

The `code` configuration value points to a folder that contains file.  This folder will be zipped up, and used to upload the source code of the lambda function that the `profile` command creates.
 
**`handler`**

The `handler` configuration value sets the created lambda function's handler value.  The above example is for a Node.js function.

**`runtime`**

The `runtime` configuration value set the runtime of the created lambda function.

**`layer_arns`**

The `profile` command will use the `layer_arn` values to automatically configure up to five layers in the lambda function it creates for profiling.  Use a value of `ELASTIC_LATEST` to build and deploy a layer with the latest lambda extension from this repo.

**`environment`**              

Use the `environment` configuration value to set any needed environment variables in the created lambda function.
