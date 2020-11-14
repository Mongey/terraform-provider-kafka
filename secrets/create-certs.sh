#!/bin/bash

set -o nounset \
  -o errexit \
  -o verbose

echo "Deleting older secrets"
set +e
rm ./*.key ./*.jks ./*.pem ./*.srl ./*.req ./*.crt ./*.csr ./*_creds
set -e

PASS=confluent

# Generate CA key
openssl req \
  -new \
  -x509 \
  -keyout snakeoil-ca-1.key \
  -out snakeoil-ca-1.crt \
  -days 365 \
  -subj '/CN=ca1.test.confluent.io/OU=TEST/O=CONFLUENT/L=PaloAlto/S=Ca/C=US' \
  -passin pass:$PASS \
  -passout pass:$PASS

# Terraform
# Private KEY
openssl genrsa \
  -des3 \
  -passout "pass:$PASS" \
  -out terraform.client.key \
  1024

# Signing Request
openssl req \
  -passin "pass:$PASS" \
  -passout "pass:$PASS" \
  -key terraform.client.key \
  -new \
  -out terraform.client.req \
  -subj '/CN=terraform.test.confluent.io/OU=TEST/O=CONFLUENT/L=PaloAlto/S=Ca/C=US'

# Signed Key
openssl x509 -req \
  -CA snakeoil-ca-1.crt \
  -CAkey snakeoil-ca-1.key \
  -in terraform.client.req \
  -out terraform-ca1-signed.pem \
  -days 9999 \
  -CAcreateserial \
  -passin "pass:$PASS"


## generate for golang

echo "generating a private key without passphrase"
openssl rsa \
  -in terraform.client.key \
  -passin "pass:$PASS" \
  -out terraform-raw-private-key.pem

echo "generating private key with passphrase"
openssl rsa \
  -aes256  \
  -passout "pass:$PASS" \
  -passin "pass:$PASS" \
  -in terraform.client.key \
  -out terraform-raw-private-key-passphrase.pem

for i in broker1
do
  echo $i
  # Create keystores
  keytool -genkey \
    -noprompt \
    -alias $i \
    -dname "CN=localhost, OU=TEST, O=CONFLUENT, L=PaloAlto, S=Ca, C=US" \
    -keystore kafka.$i.keystore.jks \
    -keyalg RSA \
    -ext SAN=dns:localhost \
    -storepass $PASS \
    -keypass $PASS

  # Create CSR, sign the key and import back into keystore
  keytool  \
    -keystore kafka.$i.keystore.jks \
    -alias $i \
    -certreq \
    -file $i.csr \
    -storepass $PASS \
    -noprompt \
    -keypass $PASS

  openssl x509 \
    -req \
    -CA snakeoil-ca-1.crt  \
    -CAkey snakeoil-ca-1.key \
    -in $i.csr \
    -out $i-ca1-signed.crt \
    -days 9999 \
    -CAcreateserial \
    -passin pass:$PASS

  keytool \
    -keystore kafka.$i.keystore.jks \
    -alias CARoot \
    -import \
    -file snakeoil-ca-1.crt \
    -storepass $PASS \
    -noprompt \
    -keypass $PASS

  keytool -keystore kafka.$i.keystore.jks \
    -alias $i \
    -import \
    -file $i-ca1-signed.crt \
    -storepass $PASS \
    -noprompt \
    -keypass $PASS

  # Create truststore and import the CA cert.
  keytool -keystore kafka.$i.truststore.jks \
    -alias CARoot \
    -import \
    -noprompt \
    -file snakeoil-ca-1.crt \
    -storepass $PASS \
    -keypass $PASS

  echo $PASS > ${i}_sslkey_creds
  echo $PASS > ${i}_keystore_creds
  echo $PASS > ${i}_truststore_creds
done
