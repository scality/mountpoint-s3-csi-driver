#!/bin/bash
# generate-test-certs.sh - Generate self-signed CA and server certificates for TLS testing
#
# Usage:
#   ./generate-test-certs.sh [OUTPUT_DIR] [HOSTNAME]
#
# Arguments:
#   OUTPUT_DIR  Directory to output certificates (default: ../../.github/scality-storage-deployment/certs)
#   HOSTNAME    Hostname for the server certificate (default: s3.scality.local)
#
# Generated files:
#   - ca.key          CA private key
#   - ca.crt          CA certificate (use this for tls.caCertSecret)
#   - server.key      Server private key
#   - server.crt      Server certificate
#   - ca-bundle.crt   Same as ca.crt, ready for Kubernetes secret creation
#
# Example:
#   ./generate-test-certs.sh ./certs s3.example.com
#   kubectl create secret generic custom-ca-cert --from-file=ca-bundle.crt=./certs/ca-bundle.crt -n kube-system

set -euo pipefail

# Get the script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Default output directory (relative to script location)
DEFAULT_OUTPUT_DIR="${SCRIPT_DIR}/../../.github/scality-storage-deployment/certs"

# Parse arguments
OUTPUT_DIR="${1:-$DEFAULT_OUTPUT_DIR}"
HOSTNAME="${2:-s3.scality.local}"

# Certificate validity in days
VALIDITY_DAYS=3650

echo "=== TLS Certificate Generation Script ==="
echo "Output directory: ${OUTPUT_DIR}"
echo "Server hostname: ${HOSTNAME}"
echo ""

# Create output directory
mkdir -p "${OUTPUT_DIR}"

# Change to output directory for easier file handling
cd "${OUTPUT_DIR}"

echo "Generating CA private key..."
openssl genrsa -out ca.key 4096

echo "Generating CA certificate..."
openssl req -new -x509 -days ${VALIDITY_DAYS} -key ca.key -out ca.crt \
    -subj "/C=US/ST=Test/L=Test/O=Test CA/CN=Test CA"

echo "Generating server private key..."
openssl genrsa -out server.key 4096

echo "Generating server certificate signing request..."
# Create a config file for SAN (Subject Alternative Names)
cat > server.cnf << EOF
[req]
default_bits = 4096
prompt = no
default_md = sha256
distinguished_name = dn
req_extensions = req_ext

[dn]
C = US
ST = Test
L = Test
O = Test Server
CN = ${HOSTNAME}

[req_ext]
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${HOSTNAME}
DNS.2 = localhost
IP.1 = 127.0.0.1
EOF

openssl req -new -key server.key -out server.csr -config server.cnf

echo "Signing server certificate with CA..."
# Create extensions file for signing
cat > server_ext.cnf << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${HOSTNAME}
DNS.2 = localhost
IP.1 = 127.0.0.1
EOF

openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
    -out server.crt -days ${VALIDITY_DAYS} -extfile server_ext.cnf

# Create ca-bundle.crt (same as ca.crt, for Kubernetes secret creation)
cp ca.crt ca-bundle.crt

# Clean up temporary files
rm -f server.csr server.cnf server_ext.cnf ca.srl

echo ""
echo "=== Certificate Generation Complete ==="
echo ""
echo "Generated files in ${OUTPUT_DIR}:"
echo "  - ca.key          CA private key (keep secure)"
echo "  - ca.crt          CA certificate"
echo "  - ca-bundle.crt   CA certificate (for Kubernetes secret)"
echo "  - server.key      Server private key"
echo "  - server.crt      Server certificate"
echo ""
echo "Certificate details:"
echo "  - Valid for: ${VALIDITY_DAYS} days"
echo "  - Server hostname: ${HOSTNAME}"
echo "  - Also valid for: localhost, 127.0.0.1"
echo ""
echo "To verify the server certificate:"
echo "  openssl verify -CAfile ${OUTPUT_DIR}/ca.crt ${OUTPUT_DIR}/server.crt"
echo ""
echo "To create a Kubernetes secret for the CSI driver:"
echo "  kubectl create secret generic custom-ca-cert \\"
echo "    --from-file=ca-bundle.crt=${OUTPUT_DIR}/ca-bundle.crt \\"
echo "    --namespace kube-system"
echo ""
echo "  kubectl create secret generic custom-ca-cert \\"
echo "    --from-file=ca-bundle.crt=${OUTPUT_DIR}/ca-bundle.crt \\"
echo "    --namespace mount-s3"
