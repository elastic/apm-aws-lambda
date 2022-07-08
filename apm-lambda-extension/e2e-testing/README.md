# End-to-End Testing

The file `e2e_test.go` contains an end-to-end test of the Elastic APM AWS Lambda extension. This test is built on top of the AWS SAM CLI, which allows running Lambda functions and their associated layers locally.

## Setup

Since this test is sensibly longer than the other unit tests, it is disabled by default. To enable it, go to `.e2e_test_config` and set the environment variable `RUN_E2E_TESTS` to `true`.
In order to run the Lambda functions locally, the following dependencies must be installed :
- [Install](https://docs.aws.amazon.com/serverless-application-model/latest/developerguide/serverless-sam-cli-install.html) the SAM CLI. Creating an AWS account is actually not required.
- Install Docker
- Install a Go Runtime

## Run

```shell
cd apm-lambda-extension/e2e-testing
go test
```

### Command line arguments
The command line arguments are presented with their default value.
```shell
-rebuild=false          # Rebuilds the Lambda function images
-lang=nodejs            # Selects the language of the Lambda function. node, java and python are supported.
-timer=20               # The timeout (in seconds) used to stop the execution of the Lambda function.
                        # Recommended values : NodeJS : 20, Python : 30, Java : 40
-java-agent-ver=1.28.4  # The version of the Java agent used when Java is selected.
```

Example :
```shell
go test -rebuild=false -lang=java -timer=40 -java-agent-ver=1.28.4
```
