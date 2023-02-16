# Image Builder contributing guide

Please refer to the [developer guide](https://www.osbuild.org/guides/developer-guide/developer-guide.html) to learn about our workflow, code style and more.

## Running the project without composer

It is possible to provide fake composer connection in order to start the service:

    PGHOST=nuc
    PGDATABASE=database
    PGUSER=user
    PGPASS=password
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
