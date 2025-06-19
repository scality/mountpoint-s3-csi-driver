# Lightweight FIO image for performance testing
FROM public.ecr.aws/docker/library/ubuntu:22.04

# Install FIO and clean up to keep image small
RUN apt-get update && \
    apt-get install -y --no-install-recommends fio ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Create directories that might be needed
RUN mkdir -p /mnt/volume1 && chmod 777 /mnt/volume1

# Add a sleep command as the default to keep container running
CMD ["sleep", "3600"]
