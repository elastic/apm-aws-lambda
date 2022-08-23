# APM AWS Lambda extension Testing

## Testing unreleased versions
If you need to test an unreleased version you can compile the package and its dependencies and make them available as AWS Lambda Layer.

### Compile package and dependencies

To run an unreleased version of this extension, you will need to ensure that your build architecture matches that of the Lambda execution environment by compiling with `GOOS=linux` and `GOARCH=amd64` if you are not running in a Linux environment.

To build the extension in the `bin/extensions` folder, run the following commands.

```bash
$ cd apm-lambda-extension
$ GOOS=linux GOARCH=amd64 go build -o bin/extensions/apm-lambda-extension main.go
$ chmod +x bin/extensions/apm-lambda-extension
```

### Layer Setup Process

Once you've compiled the extension, the next step is to make it available as an AWS Lambda Layer.  In order to do this we'll need to create a zip file with the extension binary, and then use the `lambda publish-layer-version`  command/sub-command of the AWS CLI.

The extensions .zip file should contain a root directory called `extensions/`, where the extension executables are located. In this sample project we must include the `apm-lambda-extension` binary.

To create the zip file, run the following commands from the root of your project folder.

```bash
$ cd apm-lambda-extension/bin
$ zip -r extension.zip extensions/
```

To publish the zip file as a layer, run the following command using the AWS cli (presumes you have v2 of the aws-cli installed).

Ensure that you have aws-cli v2 for the commands below.
Publish a new layer using the `extension.zip`. The output of the following command should provide you a layer arn.

```bash
aws lambda publish-layer-version \
 --layer-name "apm-lambda-extension" \
 --region <use your region> \
 --zip-file  "fileb://extension.zip"
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
