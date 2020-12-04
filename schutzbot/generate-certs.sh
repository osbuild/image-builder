#!/bin/bash
set -euxo pipefail

# Generate all X.509 certificates for the tests
# The whole generation is done in a $CADIR to better represent how osbuild-ca
# it.
CERTDIR=/etc/osbuild-composer

# create this directory b/c it doesn't exist if running in GitHub CI
if [ ! -d "$CERTDIR" ]; then
    sudo mkdir -p $CERTDIR
fi

OPENSSL_CONFIG=$(readlink -f schutzbot/openssl.cnf)
CADIR=/etc/osbuild-composer-test/ca

# The $CADIR might exist from a previous test (current Schutzbot's imperfection)
sudo rm -rf $CADIR || true
sudo mkdir -p $CADIR

pushd $CADIR
    sudo mkdir certs private
    sudo touch index.txt

    # Generate a CA.
    sudo openssl req -config "$OPENSSL_CONFIG" \
        -keyout private/ca.key.pem \
        -new -nodes -x509 -extensions osbuild_ca_ext \
        -out ca.cert.pem -subj "/CN=osbuild.org"

    # Copy the private key to the location expected by the tests
    sudo cp ca.cert.pem "$CERTDIR"/ca-crt.pem

    # Generate a composer certificate.
    sudo openssl req -config "$OPENSSL_CONFIG" \
        -keyout "$CERTDIR"/composer-key.pem \
        -new -nodes \
        -out /tmp/composer-csr.pem \
        -subj "/CN=localhost/emailAddress=osbuild@example.com" \
        -addext "subjectAltName=DNS:localhost"

    sudo openssl ca -batch -config "$OPENSSL_CONFIG" \
        -extensions osbuild_server_ext \
        -in /tmp/composer-csr.pem \
        -out "$CERTDIR"/composer-crt.pem

    # user may not exist in GitHub CI but we don't care about file
    # ownership there
    if getent passwd _osbuild-composer; then
        sudo chown _osbuild-composer "$CERTDIR"/composer-*.pem
    fi

    # Generate a worker certificate.
    sudo openssl req -config "$OPENSSL_CONFIG" \
        -keyout "$CERTDIR"/worker-key.pem \
        -new -nodes \
        -out /tmp/worker-csr.pem \
        -subj "/CN=localhost/emailAddress=osbuild@example.com" \
        -addext "subjectAltName=DNS:localhost"

    sudo openssl ca -batch -config "$OPENSSL_CONFIG" \
        -extensions osbuild_client_ext \
        -in /tmp/worker-csr.pem \
        -out "$CERTDIR"/worker-crt.pem

    # Generate a client certificate.
    sudo openssl req -config "$OPENSSL_CONFIG" \
        -keyout "$CERTDIR"/client-key.pem \
        -new -nodes \
        -out /tmp/client-csr.pem \
        -subj "/CN=client.osbuild.org/emailAddress=osbuild@example.com" \
        -addext "subjectAltName=DNS:client.osbuild.org"

    sudo openssl ca -batch -config "$OPENSSL_CONFIG" \
        -extensions osbuild_client_ext \
        -in /tmp/client-csr.pem \
        -out "$CERTDIR"/client-crt.pem

    # Client keys are used by tests to access the composer APIs. Allow all users access.
    sudo chmod 644 "$CERTDIR"/client-key.pem

    # Generate a kojihub certificate.
    sudo openssl req -config "$OPENSSL_CONFIG" \
        -keyout "$CERTDIR"/kojihub-key.pem \
        -new -nodes \
        -out /tmp/kojihub-csr.pem \
        -subj "/CN=localhost/emailAddress=osbuild@example.com" \
        -addext "subjectAltName=DNS:localhost"

    sudo openssl ca -batch -config "$OPENSSL_CONFIG" \
        -extensions osbuild_server_ext \
        -in /tmp/kojihub-csr.pem \
        -out "$CERTDIR"/kojihub-crt.pem

popd
