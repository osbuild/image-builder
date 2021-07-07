#!/bin/bash

# --------------------------------------------
# Options that must be configured by app owner
# --------------------------------------------
export APP_NAME="image-builder"  # name of app-sre "application" folder this component lives in
export COMPONENT_NAME="image-builder"  # name of app-sre "resourceTemplate" in deploy.yaml for this component
export IMAGE="quay.io/cloudservices/image-builder"

# IQE_PLUGINS="image-builder"
# IQE_MARKER_EXPRESSION="smoke"
# IQE_FILTER_EXPRESSION=""

echo "LABEL quay.expires-after=3d" >> ./Dockerfile # tag expire in 3 days

# Install bonfire repo/initialize
CICD_URL=https://raw.githubusercontent.com/RedHatInsights/bonfire/master/cicd
curl -s "$CICD_URL"/bootstrap.sh > .cicd_bootstrap.sh && source .cicd_bootstrap.sh

source "$CICD_ROOT"/build.sh
source "$CICD_ROOT"/deploy_ephemeral_env.sh
# source "$CICD_ROOT"/smoke_test.sh
