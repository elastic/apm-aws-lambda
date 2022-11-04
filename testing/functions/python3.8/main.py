import json
from elasticapm import capture_serverless

coldstart = True
@capture_serverless()
def handler(event, context):
    global coldstart
    isColdstart = coldstart
    if coldstart:
        coldstart = False
    print("Example function log", context.aws_request_id)
    return {
        "statusCode": 200,
        "body": json.dumps("Hello from Lambda!"+context.aws_request_id),
        "headers": {
            "coldstart": isColdstart,
        }
    }

