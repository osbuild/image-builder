# Image Builder contributing guide

Please refer to the [developer guide](https://www.osbuild.org/docs/developer-guide/index) to learn about our workflow, code style and more.

## Running the project locally

If you want to run project locally directly on your machine,
you can use `local.env` to pass configuration environment variables.

## Running the project without composer

It is possible to provide fake composer connection in order to start the service:

    PGHOST=nuc
    PGDATABASE=database
    PGUSER=user
    PGPASSWORD=password
    COMPOSER_CLIENT_ID=1
    COMPOSER_TOKEN_URL=http://localhost
    COMPOSER_OFFLINE_TOKEN=1
    DISTRIBUTIONS_DIR=distributions
    LOG_LEVEL=trace

Then build and run the project, or just:

    make run

## Updating package lists

`tools/generate-package-lists` can be used in combination with a `distributions/`
file to generate a package list.

If the repository requires a client tls key/cert you can supply them with
`--key` and `--cert`.
