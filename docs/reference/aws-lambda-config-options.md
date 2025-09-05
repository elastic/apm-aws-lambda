---
mapped_pages:
  - https://www.elastic.co/guide/en/apm/lambda/current/aws-lambda-config-options.html
applies_to:
  stack:
  serverless:
    observability:
---

# Configuration options [aws-lambda-config-options]

The recommended way of configuring the {{apm-lambda-ext}} and the APM agents on AWS Lambda is through the Lambda function’s environment variables.

The configuration options for the APM agents are documented in the corresponding language agents:

* [Configuration options - Node.js APM agent](apm-agent-nodejs://reference/configuration.md)
* [Configuration options - Python APM agent](apm-agent-python://reference/configuration.md)
* [Configuration options - Java APM agent](apm-agent-java://reference/configuration.md)

::::{note}
Some APM agent configuration options don’t make sense when the APM agent is running in a Lambda environment. For example, instead of using the Python APM agent configuration variable, `verify_server_cert`, you must use the `ELASTIC_APM_LAMBDA_VERIFY_SERVER_CERT` variable described below.
::::


::::{note}
APM Central configuration is not supported when using the Elastic APM AWS Lambda extension
::::



## Relevant configuration options [aws-lambda-config-relevant]

A list of relevant configuration options for the {{apm-lambda-ext}} is below.


### `ELASTIC_APM_LAMBDA_APM_SERVER` [aws-lambda-extension]

This required config option controls where the {{apm-lambda-ext}} will ship data. This should be the URL of the final APM Server destination for your telemetry.


### `ELASTIC_APM_LAMBDA_AGENT_DATA_BUFFER_SIZE` [_elastic_apm_lambda_agent_data_buffer_size]

The size of the buffer that stores APM agent data to be forwarded to the APM server. The *default* is `100`.


### `ELASTIC_APM_SECRET_TOKEN` or `ELASTIC_APM_API_KEY` [aws-lambda-config-authentication-keys]

One of these (or, alternatively, the corresponding settings for the AWS Secrets Manager IDs) needs to be set as the authentication method that the {{apm-lambda-ext}} uses when sending data to the URL configured via `ELASTIC_APM_LAMBDA_APM_SERVER`. Alternatively, you can store your APM Server credentials [using the AWS Secrets Manager](/reference/aws-lambda-secrets-manager.md) and use the [`ELASTIC_APM_SECRETS_MANAGER_SECRET_TOKEN_ID` or `ELASTIC_APM_SECRETS_MANAGER_API_KEY_ID`](#aws-lambda-config-secrets-manager-options) config options, instead. Sending data to the APM Server if none of these options is set is possible, but your APM agent must be allowed to send data to your APM server in [anonymous mode](docs-content://solutions/observability/apm/configure-anonymous-authentication.md).


### `ELASTIC_APM_SECRETS_MANAGER_SECRET_TOKEN_ID` or `ELASTIC_APM_SECRETS_MANAGER_API_KEY_ID` [aws-lambda-config-secrets-manager-options]

Instead of specifying the [`ELASTIC_APM_SECRET_TOKEN` or `ELASTIC_APM_API_KEY`](#aws-lambda-config-authentication-keys) as plain text in your Lambda environment variables, you can [use the AWS Secrets Manager](/reference/aws-lambda-secrets-manager.md) to securely store your APM authetication keys. The `ELASTIC_APM_SECRETS_MANAGER_API_KEY_ID` or `ELASTIC_APM_SECRETS_MANAGER_SECRET_TOKEN_ID` config options allow you to specify the Secrets Manager’s secret id of the stored APM API key or APM secret token, respectively, to be used by the {{apm-lambda-ext}} for authentication.

`ELASTIC_APM_SECRETS_MANAGER_SECRET_TOKEN_ID` takes precedence over [`ELASTIC_APM_SECRET_TOKEN`](#aws-lambda-config-authentication-keys), and `ELASTIC_APM_SECRETS_MANAGER_API_KEY_ID` over [`ELASTIC_APM_API_KEY`](#aws-lambda-config-authentication-keys), respectively.


### `ELASTIC_APM_SERVICE_NAME` [_elastic_apm_service_name]

The configured name of your application or service.  The APM agent will use this value when reporting data to the APM Server. If unset, the APM agent will automatically set the value based on the Lambda function name. Use this config option if you want to group multiple Lambda functions under a single service entity in APM.


### `ELASTIC_APM_DATA_RECEIVER_TIMEOUT` [aws-lambda-config-data-receiver-timeout]

```{applies_to}
product: ga 1.2.0
```

Replaces `ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS`.

The {{apm-lambda-ext}}'s timeout value, for receiving data from the APM agent. The *default* is `15s`.


### `ELASTIC_APM_DATA_RECEIVER_SERVER_PORT` [_elastic_apm_data_receiver_server_port]

The port on which the {{apm-lambda-ext}} listens to receive data from the APM agent. The *default* is `8200`.


### `ELASTIC_APM_DATA_FORWARDER_TIMEOUT` [aws-lambda-config-data-forwarder-timeout]

```{applies_to}
product: ga 1.2.0
```

Replaces `ELASTIC_APM_DATA_FORWARDER_TIMEOUT_SECONDS`.

The timeout value, for the {{apm-lambda-ext}}'s HTTP client sending data to the APM Server. The *default* is `3s`. If the extension's attempt to send APM data during this time interval is not successful, the extension queues back the data. Further attempts at sending the data are governed by an exponential backoff algorithm: data will be sent after a increasingly large grace period of 0, then circa 1, 4, 9, 16, 25 and 36 seconds, provided that the Lambda function execution is ongoing.


### `ELASTIC_APM_SEND_STRATEGY` [_elastic_apm_send_strategy]

Whether to synchronously flush APM agent data from the {{apm-lambda-ext}} to the APM Server at the end of the function invocation. The two accepted values are `background` and `syncflush`. The *default* is `syncflush`.

* The `background` strategy indicates that the {{apm-lambda-ext}} will not flush when it receives a signal that the function invocation has completed. It will instead send any remaining buffered data on the next function invocation. The result is that, if the function is not subsequently invoked for that Lambda environment, the buffered data will be lost. However, for lambda functions that have a steadily frequent load pattern the extension could delay sending the data to the APM Server to the next lambda request and do the sending in parallel to the processing of that next request. This potentially would improve both the lambda function response time and its throughput.
* The other value, `syncflush` will synchronously flush all remaining buffered APM agent data to the APM Server when the extension receives a signal that the function invocation has completed. This strategy blocks the lambda function from receiving the next request until the extension has flushed all the data. This has a negative effect on the throughput of the function, though it ensures that all APM data is sent to the APM server.


### `ELASTIC_APM_LOG_LEVEL` [_elastic_apm_log_level]

The logging level to be used by both the APM Agent and the {{apm-lambda-ext}}. Supported values are `trace`, `debug`, `info`, `warning`, `error`, `critical` and `off`.


### `ELASTIC_APM_LAMBDA_CAPTURE_LOGS` [_elastic_apm_lambda_capture_logs]

[preview] Starting in Elastic Stack version 8.5.0, the Elastic APM lambda extension supports the collection of log events by default. Log events can be viewed in {{kib}} in the APM UI. Disable log collection by setting this to `false`.


### `ELASTIC_APM_LAMBDA_VERIFY_SERVER_CERT` [_elastic_apm_lambda_verify_server_cert]

```{applies_to}
product: ga 1.3.0
```

Whether to enable {{apm-lambda-ext}} to verify APM Server's certificate chain and host name.


### `ELASTIC_APM_LAMBDA_SERVER_CA_CERT_PEM` [_elastic_apm_lambda_server_ca_cert_pem]

```{applies_to}
product: ga 1.3.0
```

The certificate passed as environment variable. To be used to verify APM Server's certificate chain if verify server certificate is enabled.


### `ELASTIC_APM_SERVER_CA_CERT_FILE` [_elastic_apm_server_ca_cert_file]

```{applies_to}
product: ga 1.3.0
```

The certificate passed as a file name available to the extension. To be used to verify APM Server's certificate chain if verify server certificate is enabled.


### `ELASTIC_APM_SERVER_CA_CERT_ACM_ID` [_elastic_apm_server_ca_cert_acm_id]

```{applies_to}
product: ga 1.3.0
```

The ARN for Amazon-issued certificate. To be used to verify APM Server's certificate chain if verify server certificate is enabled.

::::{note}
You may see errors similar to the following in {{stack}} versions less than 8.5:

```text
client error: response status code: 400
message: log: did not recognize object type
```

Users on older versions should disable log collection by setting `ELASTIC_APM_LAMBDA_CAPTURE_LOGS` to `false`.

::::



## Deprecated options [aws-lambda-config-deprecated]


### `ELASTIC_APM_DATA_RECEIVER_TIMEOUT_SECONDS` [aws-lambda-config-data-receiver-timeout-seconds]

Deprecated in: v1.2.0. Use [`ELASTIC_APM_DATA_RECEIVER_TIMEOUT`](#aws-lambda-config-data-receiver-timeout) instead.

The {{apm-lambda-ext}}'s timeout value, in seconds, for receiving data from the APM agent. The *default* is `15`.


### `ELASTIC_APM_DATA_FORWARDER_TIMEOUT_SECONDS` [aws-lambda-config-data-forwarder-timeout-seconds]

Deprecated in: v1.2.0. Use [`ELASTIC_APM_DATA_FORWARDER_TIMEOUT`](#aws-lambda-config-data-forwarder-timeout) instead.

The timeout value, in seconds, for the {{apm-lambda-ext}}'s HTTP client sending data to the APM Server. The *default* is `3`. If the extension’s attempt to send APM data during this time interval is not successful, the extension queues back the data. Further attempts at sending the data are governed by an exponential backoff algorithm: data will be sent after a increasingly large grace period of 0, then circa 1, 4, 9, 16, 25 and 36 seconds, provided that the Lambda function execution is ongoing.

