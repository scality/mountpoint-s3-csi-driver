#!/bin/bash
# Script to create lightweight FIO configuration files for CI testing

set -euo pipefail

# Create the directory if it doesn't exist
FIO_CI_DIR="tests/e2e/fio-ci"
mkdir -p "$FIO_CI_DIR"

echo "Creating lightweight FIO configs in $FIO_CI_DIR..."

# Sequential write configuration
cat > "$FIO_CI_DIR/seq_write.fio" <<EOF
[global]
name=fs_bench
bs=256k
runtime=10s
time_based
group_reporting
filename=\${FILENAME}

[sequential_write]
size=100M
rw=write
ioengine=sync
fallocate=none
create_on_open=1
fsync_on_close=1
unlink=1
EOF

# Sequential read configuration
cat > "$FIO_CI_DIR/seq_read.fio" <<EOF
[global]
name=fs_bench
bs=256k
runtime=10s
time_based
group_reporting
filename=\${FILENAME}

[sequential_read]
size=100M
rw=read
ioengine=sync
fallocate=none
EOF

# Random read configuration
cat > "$FIO_CI_DIR/rand_read.fio" <<EOF
[global]
name=fs_bench
bs=256k
runtime=10s
time_based
group_reporting
filename=\${FILENAME}

[random_read]
size=100M
rw=randread
ioengine=sync
fallocate=none
EOF

echo "Lightweight FIO configs created successfully!"
echo "Files created:"
ls -la "$FIO_CI_DIR"/*.fio
