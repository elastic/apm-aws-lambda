import json
from elasticapm import capture_serverless

coldstart = True
@capture_serverless()
def handler(event, context):
    global coldstart
    print("Example function log", context.aws_request_id)
    resp = {
        "statusCode": 200,
        "body": json.dumps("Hello from Lambda!"+context.aws_request_id),
        "headers": {
            "coldstart": isColdstart,
        }
    }
    coldstart = false
    return resp

