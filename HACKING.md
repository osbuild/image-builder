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

## Run IB with a local postgres DB

### start the DB

That script spin up postgres and delete the pod when you hit `ctrl+c`

```{bash}
#/bin/bash

# start DB
podman run -p 5432:5432 --name image-builder-db \
    --health-cmd "pg_isready -u postgres -d imagebuilder" \
    --health-interval 10s \
    --health-timeout 5s \
    --health-retries 5 \
    -e POSTGRES_USER=postgres \
    -e POSTGRES_PASSWORD=foobar \
    -e POSTGRES_DB=imagebuilder \
    docker.io/postgres
# delete pod
podman rm  image-builder-db
```

### migrate the DB

You'll need the `tern` executable for the following to work out, and the path to
the image builder clone on your machine. And you'll need to have the DB running
ofc.
This script assumes you've got tern installed using `go install` so the
executable can be fetched from the golang local storage.

```{bash}
#/bin/bash
#
export IBPATH=~/dev/image-builder
export GOPATH=~/dev/golang
export PGUSER=postgres
export PGPASSWORD=foobar
export PGDATABASE=imagebuilder
export PGHOST=localhost
export PGPORT=5432

cd $IBPATH

podman exec image-builder-db psql -U postgres -c "create database imagebuilder"

export WORKDIR=$(readlink -f internal/db/migrations-tern)
$GOPATH/bin/tern migrate -m "$WORKDIR"
```

### start IB

```
make run
```

### Run IB against staging composer

Ask a team member for credentials encrypted with you gpg key. The encrypted file
should contain these variables:

```
COMPOSER_URL=https://api.stage.openshift.com
COMPOSER_TOKEN_URL=https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token
COMPOSER_CLIENT_ID=something
COMPOSER_CLIENT_SECRET=somethin
```

Keep the encrypted file on your computer and do not left it decrypted as anyone
with these credentials can build images on stage, instead we'll use gpg to get
access to these credentials when needed.

After spinning up the postgres DB (see the section above), start IB with the
following script:

```
export IBPATH=~/dev/image-builder
export CREDSFILE=./thomas-service-account.gpg
set -a
source <(gpg --decrypt $CREDSFILE)
DISTRIBUTIONS_DIR=$IBPATH/distributions
set +a

cd $IBPATH
make run
```

## curling a local IB instance

Using curl you can request anything from your local IB instance. But you need to
have a correct set of headers for that. Use these ones:

```
-H "Content-Type: application/json"
-H "x-rh-identity: eyJlbnRpdGxlbWVudHMiOnsicmhlbCI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sImluc2lnaHRzIjp7ImlzX2VudGl0bGVkIjp0cnVlfSwic21hcnRfbWFuYWdlbWVudCI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sIm9wZW5zaGlmdCI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sImh5YnJpZCI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sIm1pZ3JhdGlvbnMiOnsiaXNfZW50aXRsZWQiOnRydWV9LCJhbnNpYmxlIjp7ImlzX2VudGl0bGVkIjp0cnVlfX0sImlkZW50aXR5Ijp7ImFjY291bnRfbnVtYmVyIjoiMDAwMDAwIiwidHlwZSI6IlVzZXIiLCJ1c2VyIjp7InVzZXJuYW1lIjoidXNlciIsImVtYWlsIjoidXNlckB1c2VyLnVzZXIiLCJmaXJzdF9uYW1lIjoidXNlciIsImxhc3RfbmFtZSI6InVzZXIiLCJpc19hY3RpdmUiOnRydWUsImlzX29yZ19hZG1pbiI6dHJ1ZSwiaXNfaW50ZXJuYWwiOnRydWUsImxvY2FsZSI6ImVuLVVTIn0sImludGVybmFsIjp7Im9yZ19pZCI6IjAwMDAwMCJ9fX0K"
```

Ofc the content type can depend on the situation.


## Updating package lists

`tools/generate-package-lists` can be used in combination with a `distributions/`
file to generate a package list.

If the repository requires a client tls key/cert you can supply them with
`--key` and `--cert`.
