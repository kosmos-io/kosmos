#!/bin/bash

WEBHOOK_NAME="kosmos-webhook"
NAMESPACE="kosmos-system"
DAYS="36500"

openssl genrsa -out ca.key 2048

openssl req -new -x509 -days ${DAYS} -key ca.key \
  -subj "/C=CN/CN=${WEBHOOK_NAME}"\
  -out ca.crt

openssl req -newkey rsa:2048 -nodes -keyout server.key \
  -subj "/C=CN/CN=${WEBHOOK_NAME}" \
  -out server.csr

openssl x509 -req \
  -extfile <(printf "subjectAltName=DNS:${WEBHOOK_NAME}.${NAMESPACE}.svc") \
  -days ${DAYS} \
  -in server.csr \
  -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out server.crt

echo
echo ">> Generating kube secrets..."
kubectl create secret tls ${WEBHOOK_NAME}-tls \
  --cert=server.crt \
  --key=server.key \
  --dry-run=client -o yaml \
  > tls-secret.yaml

echo
echo ">> MutatingWebhookConfiguration caBundle:"
cat ca.crt | base64 | fold

rm ca.crt ca.key ca.srl server.crt server.csr server.key
