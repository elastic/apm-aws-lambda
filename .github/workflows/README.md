## CI/CD

There are 5 main stages that run on GitHub actions

* Build
* Lint
* Notice
* Test
* Release

There are some other stages that run for every push on the main branches:

* [Smoke Tests](./smoke-tests.yml)

### Scenarios

* Pull Requests that are only affecting the docs files should not trigger any test or similar stages that are not required.
* Builds do not get triggered automatically for Pull Requests from contributors that are not Elasticians when need to access to any GitHub Secrets.

### How to interact with the CI?

#### On a PR basis

Once a PR has been opened then there are two different ways you can trigger builds in the CI:

1. Commit based
1. UI based, any Elasticians can force a build through the GitHub UI

#### Branches

Every time there is a merge to main or any release branches the whole workflow will compile and test every entry in the compatibility matrix for Linux and MacOS.

#### Release process

This process has been fully automated and it gets triggered when a tag release has been created, Continuous Deployment based, aka no input approval required.

#### Smoke Tests

You can run the [smoke-tests]( https://github.com/elastic/apm-aws-lambda/actions/workflows/smoke-tests.yml) using the UI if needed.

### OpenTelemetry

There is a GitHub workflow in charge to populate what the workflow run in terms of jobs and steps. Those details can be seen in [here](https://ela.st/oblt-ci-cd-stats) (**NOTE**: only available for Elasticians).
