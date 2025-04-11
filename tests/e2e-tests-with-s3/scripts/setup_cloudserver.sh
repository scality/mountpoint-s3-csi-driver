#!/bin/bash
# Script to set up CloudServer for E2E testing

set -e

echo "Setting up CloudServer for E2E testing..."

# Check if docker is available
if ! command -v docker &> /dev/null; then
    echo "Docker is required but not installed. Please install Docker first."
    exit 1
fi

# CloudServer container name
CONTAINER_NAME="s3-cloudserver-test"

# Check if container is already running
if docker ps -q -f name=$CONTAINER_NAME | grep -q .; then
    echo "CloudServer is already running."
else
    echo "Starting CloudServer..."
    
    # Run CloudServer with default configuration
    docker run -d \
        --name $CONTAINER_NAME \
        -p 8000:8000 \
        -e SCALITY_ACCESS_KEY_ID=accessKey1 \
        -e SCALITY_SECRET_ACCESS_KEY=verySecretKey1 \
        -e S3BACKEND=mem \
        zenko/cloudserver:latest
        
    echo "Waiting for CloudServer to start..."
    sleep 5
    
    # Check if container is running
    if docker ps -q -f name=$CONTAINER_NAME | grep -q .; then
        echo "CloudServer started successfully."
    else
        echo "Failed to start CloudServer."
        docker logs $CONTAINER_NAME
        exit 1
    fi
fi

# Create a test bucket
echo "Creating test bucket..."
AWS_ACCESS_KEY_ID=accessKey1 \
AWS_SECRET_ACCESS_KEY=verySecretKey1 \
aws --endpoint-url http://localhost:8000 \
    s3 mb s3://test-bucket || true

echo "CloudServer setup complete."
echo "S3 endpoint: http://localhost:8000"
echo "Access key: accessKey1"
echo "Secret key: verySecretKey1"
echo "Test bucket: test-bucket"

# This file is used to signal that setup is complete
touch /tmp/cloudserver-ready 