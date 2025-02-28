---
mapped_pages:
  - https://www.elastic.co/guide/en/apm/lambda/current/aws-lambda-overhead.html
---

# Performance impact and overhead [aws-lambda-overhead]

As described in [*APM Architecture for AWS Lambda*](/reference/index.md), using Elastic APM with AWS Lambda requires adding both the Elastic APM AWS Lambda extension and a corresponding Elastic APM agent to the Lambda runtime. These components may introduce a small overhead on the size of your function’s deployment package as well as the execution duration of your function’s invocations.


## Impact on the deployment package size [_impact_on_the_deployment_package_size]

These components contribute a little to the uncompressed deployment package size of your Lambda function. Overall, the impact of using Elastic APM on the uncompressed deployment package size of your Lambda function is less than 30MB.


## Performance impact [_performance_impact]

An advantage of the Elastic APM AWS Lambda extension architecture is that APM data dispatching is decoupled from your function’s request processing. The Elastic APM AWS Lambda extension flushes APM data to the Elastic backend *after* your function responds to the client’s request. Thus, it does not affect the latency of the client’s request. However, the extension’s flushing of APM data contributes to the overall execution time of the function invocation. The [`ELASTIC_APM_DATA_FORWARDER_TIMEOUT`](/reference/aws-lambda-config-options.md#aws-lambda-config-data-forwarder-timeout) config option with the related *exponential backoff algorithm* limits and allows to control the impact the extension may have on the function’s overall execution time.

When your function experiences a cold start, the Elastic APM AWS Lambda extension needs to be initialized and, thus, slightly increases the cold start duration (in the range of tens of milliseconds) of your function.

APM agents enrich your application’s code with measurement code that collects APM data. This measurement code introduces a small performance overhead to your application, which is usually in a negligible range. The same is true with Lambda functions. The concrete performance overhead introduced by APM agents highly depends on the configuration of the agent and on the characteristics of your function’s code. The following agent-specific documentation pages provide insights and instructions on tuning the performance the APM agents:

* [Performance Tuning - Node.js](apm-agent-nodejs://reference/performance-tuning.md)
* [Performance Tuning - Python](apm-agent-python://reference/performance-tuning.md)
* [Performance Tuning - Java](apm-agent-java://reference/overhead-performance-tuning.md)

Similar to the Elastic APM AWS Lambda extension, APM agents are initialized at cold start time. As a consequence, the APM agent’s overhead will be higher for cold starts as compared to their overhead on *warm* invocations. This effect is especially relevant for the Java APM agent on AWS Lambda. Learn more about corresponding tuning options in the [Java Agent’s AWS Lambda documentation](apm-agent-java://reference/aws-lambda.md#aws-lambda-caveats).

