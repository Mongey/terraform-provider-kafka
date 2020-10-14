#! /usr/bin/env bash

set -euox pipefail
IFS=$'\n\t'

PASSWORD='test-pass'
BROKERS=('kafka1' 'kafka2' 'kafka3')
RSA_SIZE='4096'

CN='localhost'
OU='Kitchen'
O='Krusty Krab'
L='Bikini Bottom'
ST='Bikini Atoll'
C='MH'


make_ca() {
    local ca="$1"
    local ca_key="$2"

    openssl req -x509 \
        -newkey "rsa:$RSA_SIZE" \
        -days '7300' \
        -passin "pass:$PASSWORD" \
        -passout "pass:$PASSWORD" \
        -subj "/C=$C/ST=$ST/L=$L/O=$O/OU=$OU/CN=$CN" \
        -keyout "$ca_key" \
        -out "$ca"
}

make_keystore() {
    local alias="$1"
    local keystore="kafka.$alias.keystore.jks"

    keytool -noprompt -genkey \
            -keystore "$keystore" \
            -alias "$alias" \
            -validity '7300' \
            -storepass "$PASSWORD" \
            -keypass "$PASSWORD" \
            -keyalg 'RSA' \
            -keysize "$RSA_SIZE" \
            -dname "CN=$CN, OU=$OU, O=$O, L=$L, ST=$ST, C=$C"

    echo -n "$keystore"
}

import_into_store() {
    local store="$1"
    local alias="$2"
    local file="$3"

    keytool -noprompt -import \
            -keystore "$store" \
            -alias "$alias" \
            -file "$file" \
            -storepass "$PASSWORD" \
            -keypass "$PASSWORD"
}

export_csr() {
    local keystore="$1"
    local alias="$2"
    local csr="$alias.csr"

    keytool -noprompt -certreq \
            -keystore "$keystore" \
            -alias "$alias" \
            -file "$csr" \
            -storepass "$PASSWORD" \
            -keypass "$PASSWORD"

    echo -n "$csr"
}

make_openssl_cnf() {
    broker="$1"
    cnf="$broker.cnf"

    echo "subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = $broker" > "$cnf"

    echo -n "$cnf"
}

sign_csr() {
    local csr="$1"
    local ca="$2"
    local ca_key="$3"
    local broker="$4"

    local cnf=$(make_openssl_cnf "$broker")
    local crt="$broker.crt"

    openssl x509 -req -CAcreateserial \
            -CA "$ca" \
            -CAkey "$ca_key" \
            -in "$csr" \
            -out "$crt" \
            -days '7300' \
            -passin "pass:$PASSWORD" \
            -extfile "$cnf"

    rm "$cnf"
    echo -n "$crt"
}


echo "$PASSWORD" > 'password'
make_ca 'ca.crt' 'ca.key'

import_into_store 'kafka.truststore.jks' 'CARoot' 'ca.crt'

for broker in "${BROKERS[@]}"; do
    keystore=$(make_keystore "$broker")

    csr=$(export_csr "$keystore" "$broker")
    crt=$(sign_csr "$csr" 'ca.crt' 'ca.key' "$broker")
    rm "$csr"

    import_into_store "$keystore" 'CARoot' 'ca.crt'
    import_into_store "$keystore" "$broker" "$crt"
    rm "$crt"
done

openssl genrsa -des3 -passout "pass:$PASSWORD" -out 'client.key' 4096
openssl rsa -in 'client.key' -out 'client-no-password.key' -passin "pass:$PASSWORD"

openssl req -new \
        -key 'client.key' \
        -out 'client.csr' \
        -subj "/C=$C/ST=$ST/L=$L/O=$O/OU=$OU/CN=$CN" \
        -passin "pass:$PASSWORD" \
        -passout "pass:$PASSWORD"

openssl x509 -req -CAcreateserial \
        -CA 'ca.crt' \
        -CAkey 'ca.key' \
        -in 'client.csr' \
        -out 'client.pem' \
        -days '7300' \
        -passin "pass:$PASSWORD"

rm 'client.csr' 'ca.srl'
