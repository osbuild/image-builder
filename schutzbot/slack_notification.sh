#!/bin/bash

set -eux

MESSAGE="\"Load testing on image builder finished with status *$1* $2 \n QE: @jabia \n Link to results: $CI_PIPELINE_URL \""

curl \
    -X POST \
    -H 'Content-type: application/json' \
    --data '{"text": "load test", "blocks": [ { "type": "section", "text": {"type": "mrkdwn", "text":'"$MESSAGE"'}}]}' \
    "$SLACK_WEBHOOK_URL"
