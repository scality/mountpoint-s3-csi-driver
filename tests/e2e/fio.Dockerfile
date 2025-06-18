# Lightweight FIO image for performance testing
FROM public.ecr.aws/docker/library/ubuntu:22.04

# Install FIO and clean up to keep image small
RUN apt-get update && \
    apt-get install -y --no-install-recommends fio && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Create a non-root user for running FIO tests
RUN useradd -m -u 1001 fiouser

USER fiouser
WORKDIR /home/fiouser

# Default command
CMD ["/bin/bash"]
