---
navigation_title: "Elastic APM AWS Lambda Extension"
mapped_pages:
  - https://www.elastic.co/guide/en/apm/lambda/current/aws-lambda-release-notes.html
applies_to:
  stack:
  serverless:
    observability:
---

# {{apm-lambda-ext}} release notes [elastic-apm-aws-lambda-extension-release-notes]

Review the changes, fixes, and more in each version of {{apm-lambda-ext}}. 

To check for security updates, go to [Security announcements for the Elastic stack](https://discuss.elastic.co/c/announcements/security-announcements/31).

% Release notes include only features, enhancements, and fixes. Add breaking changes, deprecations, and known issues to the applicable release notes sections. 

% ## version.next [elastic-apm-aws-lambda-extension-versionext-release-notes]
% **Release date:** Month day, year

% ### Features and enhancements [elastic-apm-aws-lambda-extension-versionext-features-enhancements]

% ### Fixes [elastic-apm-aws-lambda-extension-versionext-fixes]

## 1.7.0 [elastic-apm-aws-lambda-extension-170-release-notes]
**Release date:** February 6, 2026

### Fixes [elastic-apm-aws-lambda-extension-170-fixes]
* Upgrade Go to 1.25 [740](https://github.com/elastic/apm-aws-lambda/pull/740)

## 1.6.0 [elastic-apm-aws-lambda-extension-160-release-notes]
**Release date:** September 8, 2025

### Fixes [elastic-apm-aws-lambda-extension-160-fixes]
* Forward logs directly when invocation is over [613](https://github.com/elastic/apm-aws-lambda/pull/613)

## 1.5.8 [elastic-apm-aws-lambda-extension-158-release-notes]
**Release date:** April 10, 2025

### Fixes [elastic-apm-aws-lambda-extension-158-fixes]
* Avoid race conditions when handling data [570](https://github.com/elastic/apm-aws-lambda/pull/570)

## 1.5.7 [elastic-apm-aws-lambda-extension-157-release-notes]
**Release date:** July 25, 2024

### Fixes [elastic-apm-aws-lambda-extension-157-fixes]
* Create a new bytes reader instead of sharing bytes buffer [511](https://github.com/elastic/apm-aws-lambda/pull/511)
* Do not close log processing channel if logs api is disabled [512](https://github.com/elastic/apm-aws-lambda/pull/512)
* Only flush logs if logs collection is enabled [510](https://github.com/elastic/apm-aws-lambda/pull/510)

## 1.5.6 [elastic-apm-aws-lambda-extension-156-release-notes]
**Release date:** July 24, 2024

### Fixes [elastic-apm-aws-lambda-extension-156-fixes]
* Ensure buffered logs are flushed [509](https://github.com/elastic/apm-aws-lambda/pull/509)

## 1.5.5 [elastic-apm-aws-lambda-extension-155-release-notes]
**Release date:** June 25, 2024

This release contains no user-facing changes.

## 1.5.4 [elastic-apm-aws-lambda-extension-154-release-notes]
**Release date:** April 26, 2024

This release contains no user-facing changes.

## 1.5.3 [elastic-apm-aws-lambda-extension-153-release-notes]
**Release date:** January 22, 2024

### Features and enhancements [elastic-apm-aws-lambda-extension-153-features-enhancements]
* Add `ELASTIC_APM_LAMBDA_DISABLE_LOGS_API` env var to disable logs api [434](https://github.com/elastic/apm-aws-lambda/pull/434)

## 1.5.2 [elastic-apm-aws-lambda-extension-152-release-notes]
**Release date:** January 11, 2024

### Features and enhancements [elastic-apm-aws-lambda-extension-152-features-enhancements]
* Use sandbox.localdomain as logsapi address [425](https://github.com/elastic/apm-aws-lambda/pull/425)

## 1.5.1 [elastic-apm-aws-lambda-extension-151-release-notes]
**Release date:** October 6, 2023

### Fixes [elastic-apm-aws-lambda-extension-151-fixes]
* Fix incorrect proxy transaction handling at shutdown due to not flushing the data before processing shutdown event. [412](https://github.com/elastic/apm-aws-lambda/pull/412).

## 1.5.0 [elastic-apm-aws-lambda-extension-150-release-notes]
**Release date:** September 13, 2023

### Features and enhancements [elastic-apm-aws-lambda-extension-150-features-enhancements]
* Use User-Agent header with Lambda extension version and propagate info from apm agents [404](https://github.com/elastic/apm-aws-lambda/pull/404)

### Fixes [elastic-apm-aws-lambda-extension-150-fixes]
* Log a warning, instead of failing a Lambda function, if auth retrieval from AWS Secrets Manager fails. Reporting APM data will not work, but the Lambda function invocations will proceed. [401](https://github.com/elastic/apm-aws-lambda/pull/401)

## 1.4.0 [elastic-apm-aws-lambda-extension-140-release-notes]
**Release date:** May 3, 2023

### Features and enhancements [elastic-apm-aws-lambda-extension-150-features-enhancements]
* {applies_to}`product: preview` Allow metadata in register transaction [384](https://github.com/elastic/apm-aws-lambda/pull/384)

## 1.3.1 [elastic-apm-aws-lambda-extension-131-release-notes]
**Release date:** April 4, 2023

### Fixes [elastic-apm-aws-lambda-extension-131-fixes]
* Print response body on error if decoding fails [382](https://github.com/elastic/apm-aws-lambda/pull/382)

## 1.3.0 [elastic-apm-aws-lambda-extension-130-release-notes]
**Release date:** April 22, 2023

### Features and enhancements [elastic-apm-aws-lambda-extension-130-features-enhancements]
* {applies_to}`product: preview` Create proxy transaction with error results if not reported by agent [315](https://github.com/elastic/apm-aws-lambda/pull/315)
* Wait for the final platform report metrics on shutdown [347](https://github.com/elastic/apm-aws-lambda/pull/347)
* Process platform report metrics when extension is lagging [358](https://github.com/elastic/apm-aws-lambda/pull/358)
* Add TLS support [357](https://github.com/elastic/apm-aws-lambda/pull/357)

## 1.2.0 [elastic-apm-aws-lambda-extension-120-release-notes]
**Release date:** November 1, 2022

### Features and enhancements [elastic-apm-aws-lambda-extension-120-features-enhancements]
* Parse and log APM Server error responses, and backoff on critical errors [281](https://github.com/elastic/apm-aws-lambda/pull/281)
* Disable CGO to prevent libc/ABI compatibility issues [292](https://github.com/elastic/apm-aws-lambda/pull/292)
* Deprecate `ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS` in favour of `ELASTIC_APM_DATA_RECEIVER_TIMEOUT` [294](https://github.com/elastic/apm-aws-lambda/pull/294)
* Log shutdown reason on exit [297](https://github.com/elastic/apm-aws-lambda/pull/297)
* Add support for collecting and shipping function logs to APM Server [303](https://github.com/elastic/apm-aws-lambda/pull/303)
* Batch data collected from lambda logs API before sending to APM Server [314](https://github.com/elastic/apm-aws-lambda/pull/314)

### Fixes [elastic-apm-aws-lambda-extension-120-fixes]
* Fix possible data corruption while processing multiple log events [309](https://github.com/elastic/apm-aws-lambda/pull/309)

## 1.1.0 [elastic-apm-aws-lambda-extension-110-release-notes]
**Release date:** August 24, 2022

### Features and enhancements [elastic-apm-aws-lambda-extension-110-features-enhancements]
* Added support for Secret Manager [208](https://github.com/elastic/apm-aws-lambda/pull/208)
* Added support for Lambda platform metrics [202](https://github.com/elastic/apm-aws-lambda/pull/202)
* Migrated to AWS SDK for Go v2 [232](https://github.com/elastic/apm-aws-lambda/pull/232)
* Make buffer size for agent data configurable [262](https://github.com/elastic/apm-aws-lambda/pull/262)
* Add support for reproducible builds [237](https://github.com/elastic/apm-aws-lambda/pull/237)
* Improve extension client error messages [259](https://github.com/elastic/apm-aws-lambda/pull/259)

### Fixes [elastic-apm-aws-lambda-extension-110-fixes]
* Log a warning when authentication with APM Server fails [228](https://github.com/elastic/apm-aws-lambda/pull/228)
* Handle http.ErrServerClosed correctly [234](https://github.com/elastic/apm-aws-lambda/pull/234)
* Handle main loop errors correctly [252](https://github.com/elastic/apm-aws-lambda/pull/252)
* Avoid sending corrupted compressed data to APM Server [257](https://github.com/elastic/apm-aws-lambda/pull/257)
* Avoid creating http transports on each info request [260](https://github.com/elastic/apm-aws-lambda/pull/260)
* Randomise the initial grace period to avoid collisions [240](https://github.com/elastic/apm-aws-lambda/pull/240)
* Handle metadata errors correctly [254](https://github.com/elastic/apm-aws-lambda/pull/254)
* Always flush data to APM server before shutting down and avoid concurrent access to data channel [258](https://github.com/elastic/apm-aws-lambda/pull/258)

## 1.0.2 [elastic-apm-aws-lambda-extension-102-release-notes]
**Release date:** June 9, 2022

### Fixes [elastic-apm-aws-lambda-extension-102-fixes]
* Only add executables to extension [216](https://github.com/elastic/apm-aws-lambda/pull/216)

## 1.0.1 [elastic-apm-aws-lambda-extension-101-release-notes]
**Release date:** June 3, 2022

### Features and enhancements [elastic-apm-aws-lambda-extension-101-features-enhancements]
* Add support for building and pushing docker images [199](https://github.com/elastic/apm-aws-lambda/pull/199)

## 1.0.0 [elastic-apm-aws-lambda-extension-100-release-notes]
**Release date:** April 26, 2022

### Features and enhancements [elastic-apm-aws-lambda-extension-100-features-enhancements]
* First stable release of the Elastic APM AWS Lambda extension.

