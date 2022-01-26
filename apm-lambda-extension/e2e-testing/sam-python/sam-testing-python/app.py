import json
import os

# import requests
import elasticapm
from elasticapm import capture_serverless

@capture_serverless()
def lambda_handler(event, context):
    elasticapm.set_transaction_name(os.environ.get('APM_AWS_EXTENSION_TEST_UUID'))
    return {
        "statusCode": 200,
        "body": json.dumps({
            "message": "hello world",
            # "location": ip.text.replace("\n", "")
        }),
    }
