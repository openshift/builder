#!/bin/sh

clusterCA="/var/run/configs/openshift.io/pki/tls-ca-bundle.pem"

if [ -s "$clusterCA" ]; then
    echo "Adding cluster TLS certificate authority to trust store"
    cp -f "$clusterCA" /etc/pki/ca-trust/source/anchors/cluster-ca-bundle.pem
    update-ca-trust extract
fi

exec ionice -c3 "$@"
