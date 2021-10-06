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