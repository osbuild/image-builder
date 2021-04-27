# Running locally

Image Builder needs a database running, the easiest way is to run a container like so:

```
sudo podman run -p 5432:5432 --name image-builder-db --health-cmd "pg_isready -u postgres -d imagebuilder"
--health-interval 10s --health-timeout 5s --health-retries 5 -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=foobar -e
POSTGRES_DB=imagebuilder postgres
```

Run `make build` and migrate the db (this only needs to be done once) to the latest version:

```
PGHOST=localhost PGPORT=5432 PGUSER=postgres PGPASSWORD=foobar PGDATABASE=imagebuilder
MIGRATIONS_DIR=/home/sanne/workstuff/image-builder/internal/db/migrations/ ./image-builder-migrate-db
```

Start Image Builder:
```
PGHOST=localhost PGPORT=5432 PGUSER=postgres PGPASSWORD=foobar PGDATABASE=imagebuilder ALLOWED_ORG_IDS="*"
DISTRIBUTIONS_DIR="/home/sanne/workstuff/image-builder/distributions" ./image-builder
```

To compose images locally, and if you have Composer running locally, add these to the environment:

```
OSBUILD_URL="https://localhost:8085" OSBUILD_CA_PATH="/etc/osbuild-composer/ca-crt.pem"
OSBUILD_CERT_PATH="/etc/osbuild-composer/client-crt.pem" OSBUILD_KEY_PATH="/etc/osbuild-composer/client-key.pem"
```

# Testing

This directory contains automated integration tests for Image
Builder. Infrastructure related files can be found under schutzbot.

## Unit tests

```
go clean -testcache
go test -v ./...
```

## Integration test

It's recommended to run these inside of a vm as the scripts make extensive
changes to the host. Running integration test requires specific environment
variables to be set on the system. The specific list for each supported cloud
provider can be found in the following sub-sections.

1. Build the docker image:

`sudo podman build --security-opt "label=disable" -t image-builder -f
distribution/Dockerfile-ubi .`

2. Setup Osbuild Composer and start the Image Builder container

Call `schutzbot/deploy.sh`. This will install composer, generate certificates,
and start the image-builder container.

3. Call `test/cases/api.sh <cloud_provider>` to run the integration tests for
a specific cloud provider. Valid values for `<cloud_provider>` are `aws`,
`azure` and `gcp`.

### Setting up AWS integration test

The following environment variables are required

- `AWS_REGION`
- `AWS_BUCKET`
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_API_TEST_SHARE_ACCOUNT`

To execute the AWS integration test, complete steps 1-2 from the *Integration test*
section and run `test/cases/api.sh aws`.

### Setting up Azure integration test

The following environment variables are required

- `AZURE_TENANT_ID`
- `AZURE_SUBSCRIPTION_ID`
- `AZURE_RESOURCE_GROUP`
- `AZURE_LOCATION`
- `AZURE_CLIENT_ID`
- `AZURE_CLIENT_SECRET`

To execute the AWS integration test, complete steps 1-2 from the *Integration test*
section and run `test/cases/api.sh azure`.

#### Setting up GCP integration test

The following environment variables are required:

- `GOOGLE_APPLICATION_CREDENTIALS` - path to [Google authentication credentials][gcp_creds] file.
- `GCP_REGION`
- `GCP_BUCKET`
- `GCP_API_TEST_SHARE_ACCOUNT`

To execute the AWS integration test, complete steps 1-2 from the *Integration test*
section and run `test/cases/api.sh gcp`.

[gcp_creds]: https://cloud.google.com/docs/authentication/getting-started#setting_the_environment_variable

## Code coverage

Coverage report is available from
[CodeCov](https://codecov.io/github/osbuild/image-builder/).
