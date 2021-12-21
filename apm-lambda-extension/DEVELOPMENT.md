
## Releasing

Releasing a version of the Lambda Extension is currently a three step manual process.

1. Tag the Release
2. Create the Build Artifacts
3. Add a Release via the Github UI

### Tag the Release

First, tag the release via your preferred tagging method.  Tagging a release (v0.0.2) via the command line looks something like this.

    % git clone git@github.com:elastic/apm-aws-lambda.git
    # ...
    % cd apm-aws-lambda
    % git checkout main
    % git tag v0.0.2
    % git push --tags
    Total 0 (delta 0), reused 0 (delta 0), pack-reused 0
    To github.com:elastic/apm-aws-lambda.git
     * [new tag]         v0.0.2 -> v0.0.2


### Create the Build Artifacts

Next, create the build artifacts for the release.  These are go binaries of the Lambda Extension, built for both Intel and ARM architectures.

If you were creating the build artifacts for the v0.0.2 release, that might look something like this


    % GOARCH=arm64 make build
    % GOARCH=arm64 make zip
    % ls -1 bin/arm64.zip
    bin/arm64.zip
    % mv bin/arm64.zip bin/v0-0-2-linux-arm64.zip

    % GOARCH=amd64 make build
    % GOARCH=amd64 make zip
    % ls -lh bin/amd64.zip
    % mv bin/amd64.zip bin/v0-0-2-linux-amd64.zip

###  Add a Release via the Github UI

You can add a release via the GitHub UI by

1. Navigating to [the repo homepage](https://github.com/elastic/apm-aws-lambda/)

2. Clicking on [create new release](https://github.com/elastic/apm-aws-lambda/releases/new)

3. Selecting the Tag (tagged above)

4. Entering a release title, description, and binaries

5. Clicking on the Publish Release button
