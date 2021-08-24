#!/usr/bin/bash

#
# The image-builder Load test

set -euxo pipefail

# install locust
sudo dnf install -y python3 python3-pip gcc python3-devel make
sudo pip3 install locust

# Configuration of the load test

USERS=20            # number of concurrent users
SPAWN_RATE=1        # warm-up rate at which users arrive (per second)
DURATION=60         # duration of the test in "UNIT" below
DURATION_UNIT="s"

# By default, debug tags are disabled

#INCLUDE_TAGS="" #set to --tags tag1 tag2 ... tagn
EXCLUDE_TAGS="debug"

# The URL to test, it must have the path to the application + api version in it

PROTOCOL="https"
URL="console.stage.redhat.com/api/image-builder/v1"

# Set of thresholds for the load test validation

export LT_FAIL_RATIO="0.01"
export LT_MEAN_RESPONSE_TIME="200"
export LT_MEDIAN_RESPONSE_TIME="280"
export LT_PERCENTILE_95_RESPONSE_TIME="500"

# Finally, run the load test

locust -f test/cases/load_test.py \
     -H \
      "${PROTOCOL}://${LOAD_TEST_ETHEL_LOGIN}:${LOAD_TEST_ETHEL_PASSWORD}@${URL}" \
      --headless \
      --users "${USERS}" \
      --spawn-rate "${SPAWN_RATE}" \
      --run-time "${DURATION}${DURATION_UNIT}" \
      --exclude-tags "${EXCLUDE_TAGS}" # \
     #--tags "${INCLUDE_TAGS}
