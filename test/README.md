# Running locally

Image Builder needs a database running, the easiest way is to run a container like so:

```
sudo podman run -p 5432:5432 --name image-builder-db --health-cmd "pg_isready -u postgres -d imagebuilder" --health-interval 10s --health-timeout 5s --health-retries 5 -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=foobar -e POSTGRES_DB=imagebuilder postgres
```

Open `image-builder-migrate-db-tern.go` file (under `image-builder/cmd` folder) and update 2 variables:
1) TernExecutable with path to tern executable location.
   If you don't have tern tool you can install it with -
   `go install github.com/jackc/tern/v2@latest`
2) TernMigrationsDir with path to migration dir -
   `$HOME/image-builder/internal/db/migrations-tern`


Run `make build` and migrate the db (this only needs to be done once) to the latest version:

```
PGHOST=localhost PGPORT=5432 PGUSER=postgres PGPASSWORD=foobar PGDATABASE=imagebuilder MIGRATIONS_DIR=$HOME/image-builder/internal/db/migrations-tern ./image-builder-migrate-db-tern
```

To compose images locally, and if you have Composer running locally, add these to the environment:

```
OSBUILD_URL="https://localhost:8085" OSBUILD_CA_PATH="/etc/osbuild-composer/ca-crt.pem"
OSBUILD_CERT_PATH="/etc/osbuild-composer/client-crt.pem" OSBUILD_KEY_PATH="/etc/osbuild-composer/client-key.pem"
```

Otherwise set some other values and Start Image Builder:
```
PGHOST=localhost PGPORT=5432 PGUSER=postgres PGPASSWORD=foobar PGDATABASE=imagebuilder DISTRIBUTIONS_DIR="$HOME/image-builder/distributions" COMPOSER_TOKEN_URL=test COMPOSER_CLIENT_ID=test COMPOSER_CLIENT_SECRET=test ./image-builder
```

# Testing

This directory contains automated integration tests for Image Builder. Infrastructure related files
can be found under schutzbot.

## Unit tests

Run the following from **the root directory** of the repository:

```
go clean -testcache
go test ./...
```

Some tests do require database and start a postgres container using podman or docker.


## Unit integration tests

You can run tests which are executed on CI under name "DB tests" locally, just keep in mind this drops public schema completely.

```
TERN_MIGRATIONS_DIR=../../internal/db/migrations-tern/ go test -tags integration ./...
```

## End-to-end tntegration tests

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
- `V2_AWS_ACCESS_KEY_ID`
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
- `V2_AZURE_CLIENT_ID`
- `V2_AZURE_CLIENT_SECRET`

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

## Load Testing

The load testing runs on the CI machine at a fixed schedule.

### Installation

To execute it manually, you need to install locust:

```
# install locust
sudo dnf install -y python3 python3-pip gcc python3-devel make
sudo pip3 install locust
```

### Run the tests

And then you can run the test you'll need:

* to have a user and a password to access console.stage.redhat.com
* to be on the redhat VPN.
* to have the corp proxy setup

If you don't have a user on ethel, you can create one here:
http://account-manager-stage.app.eng.rdu2.redhat.com/#create
Note that your account name should not be the same as you kerberos password.

The proxy to use is this one: http://hdn.corp.redhat.com/proxy.pac

#### Headless run:

```
locust -f test/cases/load_test.py -H https://$USER:$PASSWORD@console.stage.redhat.com/api/image-builder/v1  --users 20 --spawn-rate 1 --run-time 30s --headless
```

#### Headed run:

```
locust -f test/cases/load_test.py -H https://$USER:$PASSWORD@console.stage.redhat.com/api/image-builder/v1
```

This mode will give you a local webpage to visit where you can configure the
number of users you want and also will compute you nice graphs.

