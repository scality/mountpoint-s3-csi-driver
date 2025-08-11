# Lightweight Dockerfile for Pre-commit and MkDocs Development
FROM ubuntu:22.04

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive
ENV TERM=xterm-256color

# Set up workspace
WORKDIR /workspace

# Install minimal system dependencies for pre-commit and mkdocs
RUN apt-get update && apt-get install -y \
    # Basic utilities
    curl \
    git \
    # Python and pip
    python3 \
    python3-pip \
    python3-venv \
    # SSL/TLS for downloads
    ca-certificates \
    # Clean up
    && rm -rf /var/lib/apt/lists/*

# Set up Python virtual environment and install requirements
COPY requirements.txt /tmp/requirements.txt
RUN python3 -m venv /workspace/.venv \
    && /workspace/.venv/bin/pip install --upgrade pip \
    && /workspace/.venv/bin/pip install -r /tmp/requirements.txt

# Set up shell environment
RUN echo 'source /workspace/.venv/bin/activate' >> /root/.bashrc \
    && echo 'alias ll="ls -la"' >> /root/.bashrc \
    && echo 'cd /workspace' >> /root/.bashrc

# Expose port for mkdocs dev server
EXPOSE 8000

# Default command
CMD ["/bin/bash"]
