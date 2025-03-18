# APM AWS Lambda extension Testing

## Testing unreleased versions
If you need to test an unreleased version you can compile the package and its dependencies and make them available as AWS Lambda Layer.

### Compile package and dependencies

To run an unreleased version of this extension, you will need to ensure that your build architecture matches that of the Lambda execution environment by compiling with `GOOS=linux` and `GOARCH=amd64` if you are not running in a Linux environment.

To build the extension in the `bin/extensions` folder, run the following commands.

```bash
$ make build
```

### Layer Setup Process

Once you've compiled the extension, the next step is to make it available as an AWS Lambda Layer.  In order to do this we'll need to create a zip file with the extension binary, and then use the `lambda publish-layer-version`  command/sub-command of the AWS CLI.

The extensions .zip file should contain a root directory called `extensions/`, where the extension executables are located. In this sample project we must include the `apm-lambda-extension` binary.

To create the zip file, run the following commands from the root of your project folder.

```bash
$ make zip
```

To publish the zip file as a layer, run the following command using the AWS cli (presumes you have v2 of the aws-cli installed).

Ensure that you have aws-cli v2 for the commands below.
Publish a new layer using the `extension.zip`. The output of the following command should provide you a layer arn.

```bash
aws lambda publish-layer-version \
 --layer-name "apm-lambda-extension" \
 --region <use your region> \
 --description "AWS Lambda Extension Layer for Elastic APM" \
 --license "Apache-2.0" \
 --zip-file  "fileb://./bin/extension.zip"
```

The output from the above command will include a `LayerVersionArn` field, which contains the unique string identifier for your layer.  The will look something like the following.

    `"LayerVersionArn": "arn:aws:lambda:<region>:123456789012:layer:<layerName>:1"`

This is the string you'll enter in the AWS Lambda Console to add this layer to your Lambda function.

### One Step Build

The `Makefile` also provides a `build-and-publish` command which will perform the above steps for use, using ENV variable for credentials and other information.

    $ ELASTIC_LAYER_NAME=apm-lambda-extension \
    AWS_DEFAULT_REGION=us-west-2 \
    AWS_ACCESS_KEY_ID=A...X \
    AWS_SECRET_ACCESS_KEY=h...E \
    make build-and-publish

## Running smoketests

If you haven't yet, go to https://elastic-observability.signin.aws.amazon.com/console ,
login and and create an API key (drop down your login on the right top corner, select "Security credentials").

On the command line, export the environment variables
```
export EC_API_KEY="..."
export AWS_PROFILE=observability
```
The `AWS_PROFILE` value is the profile that you created with `aws configure --profile observability`.
In `~/.aws/config` there should be an entry like this
```
[profile observability]
region = eu-central-1
```

Now run the smoke tests with
```
make smoketest/run
```

If you want to keep the AWS Lambda test environment for manual testing
```
make smoketest/run SKIP_DESTROY=1
```
Now you can find your environment under Lambda/Functions in the AWS console.

Destroy the environment with
```
make smoketest/destroy
```
