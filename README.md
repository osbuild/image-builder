# image-builder


## Testing

### Preparation
1. Clone the following repositories:
    * https://github.com/osbuild/image-builder

2. Setting up osbuild-composer(-api)

    Move to image-builder folder. The easiest way to do this is to call schutzbots/provision-composer.sh. This will install composer, generate the needed certs, and put the configuration in place.


### Unit test

* In image-builder folder, run all the unit tests:

    ```
    # go clean -testcache # Clean cache before rerun unit tests
    # go test ./... # Recursively run all the unit tests
    ```

### Integration test

* Run the following script to easily run the integration test:
    ```
    # test/run_integration_test.sh
    ```

* Detail steps:

1. Build image-builder docker image.

    Call schutzbots/build.sh. It will build image-builder and image-builder-tests packages, install them into your testbed, and build an image-builder docker image
    ```
    # export WORKSPACE=.
    # export JOB_NAME=
    # sudo schutzbot/build.sh
    ```

2. Run integration test

    Call schutzbot/run_tests.sh. It will start a image-builder container and run image-builder-tests. Or alternatively, run the integration test manually:

    ```
    # sudo podman run -d --pull=never --security-opt "label=disable" --net=host \
     -e LISTEN_ADDRESS=localhost:8087 -e OSBUILD_URL=https://localhost:443/api/composer/v1 \
     -e OSBUILD_CA_PATH=/etc/osbuild-composer/ca-crt.pem \
     -e OSBUILD_CERT_PATH=/etc/osbuild-composer/client-crt.pem \
     -e OSBUILD_KEY_PATH=/etc/osbuild-composer/client-key.pem \
     -v /etc/osbuild-composer:/etc/osbuild-composer \
     image-builder # Start image-builder container
    # go test ./cmd/... -tags=integration # Run integration test
    ```

### Code coverage

* Coverage report is available from
[CodeCov](https://codecov.io/github/osbuild/image-builder/).
