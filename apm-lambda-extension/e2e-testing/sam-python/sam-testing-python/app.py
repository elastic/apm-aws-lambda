import json
import os

# import requests
import elasticapm
from elasticapm import capture_serverless

@capture_serverless()
def lambda_handler(event, context):
    client = elasticapm.get_client()
    client.begin_transaction('logging')
    elasticapm.set_transaction_name(os.environ.get('APM_AWS_EXTENSION_TEST_UUID'))
    client.end_transaction("A Python Lambda function sends you its regards")
    return {
        "statusCode": 200,
        "body": json.dumps({
            "message": "hello world",
            # "location": ip.text.replace("\n", "")
        }),
    }
