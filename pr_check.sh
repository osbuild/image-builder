#!/bin/bash

# --------------------------------------------
# Options that must be configured by app owner
# --------------------------------------------
export APP_NAME="image-builder-crc"                # name of app-sre "application" folder this component lives in
export COMPONENT_NAME="image-builder"              # name of app-sre "resourceTemplate" in deploy.yaml for this component
export IMAGE="quay.io/cloudservices/image-builder" # image location on quay

export IQE_PLUGINS="image-builder"        # name of the IQE plugin for this app.
export IQE_CJI_TIMEOUT="60m"              # This is the time to wait for smoke test to complete or fail
export IQE_MARKER_EXPRESSION="auth_debug" # run only tests marked by be_pr_check
export IQE_ENV="ephemeral"                # run only api test
export IQE_IMAGE_TAG="image-builder"
export DOCKERFILE="distribution/Dockerfile-ubi"
export EXTRA_DEPLOY_ARGS="sources unleash-proxy"
export REF_ENV="insights-stage"

# Install bonfire repo/initialize
# https://raw.githubusercontent.com/RedHatInsights/bonfire/master/cicd/bootstrap.sh
# This script automates the install / config of bonfire
CICD_URL=https://raw.githubusercontent.com/RedHatInsights/bonfire/master/cicd
curl -s "$CICD_URL"/bootstrap.sh >.cicd_bootstrap.sh && source .cicd_bootstrap.sh

# The contents of build.sh can be found at:
# https://raw.githubusercontent.com/RedHatInsights/bonfire/master/cicd/build.sh
# This script is used to build the image that is used in the PR Check
source "$CICD_ROOT"/build.sh

# The contents of this script can be found at:
# https://raw.githubusercontent.com/RedHatInsights/bonfire/master/cicd/deploy_ephemeral_env.sh
# This script is used to deploy the ephemeral environment for smoke tests.
source "$CICD_ROOT"/deploy_ephemeral_env.sh

# Run smoke tests using a ClowdJobInvocation (preferred)
# The contents of this script can be found at:
# https://raw.githubusercontent.com/RedHatInsights/bonfire/master/cicd/cji_smoke_test.sh
source "$CICD_ROOT"/cji_smoke_test.sh

# Post a comment with test run IDs to the PR
# The contents of this script can be found at:
# https://raw.githubusercontent.com/RedHatInsights/bonfire/master/cicd/post_test_results.sh
source "$CICD_ROOT"/post_test_results.sh
