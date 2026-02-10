
## Release Procedure 

### Preparing a Release

1. Update the [`CHANGELOG.asciidoc`](CHANGELOG.asciidoc), by adding a new version heading (`==== 1.x.x - yyyy/MM/dd`) and changing the base tag of the Unreleased comparison URL
2. Ensure all changes are merged into github.com/elastic/apm-aws-lambda@main
3. Create a test plan for any changes that require manual testing. Ensure that the automatic smoke test on the latest commit is successful.
4. Trigger a release after succesful testing.

### Trigger a Release

Releasing a version of the Elastic APM AWS Lambda extension requires a tag release.

Tag the release via your preferred tagging method.  Tagging a release (v1.1.0) via the command line looks something like this.

    % git clone git@github.com:elastic/apm-aws-lambda.git
    # ...
    % cd apm-aws-lambda
    % git checkout main
    % git tag v1.1.0
    % git push --tags
    Total 0 (delta 0), reused 0 (delta 0), pack-reused 0
    To github.com:elastic/apm-aws-lambda.git
     * [new tag]         v1.1.0 -> v1.1.0

This will trigger a build in the CI that will create the Build Artifacts
and a Release in the Github UI.

### Testing release locally

> [!INFO]
> Configure your AWS credentials using https://github.com/okta/okta-aws-cli
> Use gh cli to configure the GITHUB_TOKEN

1. export GORELEASER_CURRENT_TAG=v0.0.99999
2. export GITHUB_TOKEN=$(gh auth token)
3. git tag "${GORELEASER_CURRENT_TAG}" -f
3. make release-skip-docker

#### Further details

What does `GORELEASER_CURRENT_TAG` mean?

  See https://goreleaser.com/cookbooks/set-a-custom-git-tag
