#!/usr/bin/env bash
set -euo pipefail

# Generate self-signed TLS certificates for the ingress.
# Usage: ./generate-certs.sh [FQDN]
# If no FQDN is provided, generates a wildcard cert valid for any hostname.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERT_DIR="${SCRIPT_DIR}/certs"
FQDN="${1:-first-responder.local}"

mkdir -p "${CERT_DIR}"

echo "==> Generating self-signed CA..."
openssl genrsa -out "${CERT_DIR}/ca.key" 4096 2>/dev/null

openssl req -x509 -new -nodes \
  -key "${CERT_DIR}/ca.key" \
  -sha256 -days 365 \
  -out "${CERT_DIR}/ca.crt" \
  -subj "/CN=First Responder CA/O=FirstResponder"

echo "==> Generating server certificate for: ${FQDN}"
openssl genrsa -out "${CERT_DIR}/tls.key" 4096 2>/dev/null

openssl req -new \
  -key "${CERT_DIR}/tls.key" \
  -out "${CERT_DIR}/tls.csr" \
  -subj "/CN=${FQDN}/O=FirstResponder"

cat > "${CERT_DIR}/san.cnf" <<EOF
[req]
distinguished_name = req_distinguished_name
[req_distinguished_name]
[v3_ext]
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage=digitalSignature,keyEncipherment
extendedKeyUsage=serverAuth
subjectAltName=DNS:${FQDN},DNS:*.${FQDN},DNS:localhost
EOF

openssl x509 -req \
  -in "${CERT_DIR}/tls.csr" \
  -CA "${CERT_DIR}/ca.crt" \
  -CAkey "${CERT_DIR}/ca.key" \
  -CAcreateserial \
  -out "${CERT_DIR}/tls.crt" \
  -days 365 \
  -sha256 \
  -extensions v3_ext \
  -extfile "${CERT_DIR}/san.cnf" 2>/dev/null

rm -f "${CERT_DIR}/tls.csr" "${CERT_DIR}/ca.srl" "${CERT_DIR}/san.cnf"

echo "==> Certificates generated in: ${CERT_DIR}/"
echo "    CA:   ${CERT_DIR}/ca.crt"
echo "    Cert: ${CERT_DIR}/tls.crt"
echo "    Key:  ${CERT_DIR}/tls.key"
echo ""
echo "==> Now run: kubectl apply -k ."
