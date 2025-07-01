# Multi-stage Dockerfile for Scality S3 CSI Driver Development Environment
FROM ubuntu:22.04

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive
ENV TERM=xterm-256color

# Set up workspace
WORKDIR /workspace

# Install system dependencies
RUN apt-get update && apt-get install -y \
    # Basic utilities
    curl \
    wget \
    git \
    unzip \
    tar \
    gzip \
    jq \
    vim \
    nano \
    tree \
    watch \
    htop \
    # Build tools
    build-essential \
    ca-certificates \
    gnupg \
    lsb-release \
    software-properties-common \
    # Python and pip
    python3 \
    python3-pip \
    python3-venv \
    # Network tools
    net-tools \
    iputils-ping \
    # SSL/TLS
    openssl \
    # Clean up
    && rm -rf /var/lib/apt/lists/*

# Install Go 1.24.0 (matching go.mod requirement)
RUN wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz \
    && rm go1.24.0.linux-amd64.tar.gz

# Install Node.js and npm (for some development tools)
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs

# Install Docker
RUN curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg \
    && echo "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null \
    && apt-get update \
    && apt-get install -y docker-ce-cli \
    && rm -rf /var/lib/apt/lists/*

# Install kubectl
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
    && chmod +x kubectl \
    && mv kubectl /usr/local/bin/

# Install Helm
RUN curl https://baltocdn.com/helm/signing.asc | gpg --dearmor | tee /usr/share/keyrings/helm.gpg > /dev/null \
    && echo "deb [arch=amd64 signed-by=/usr/share/keyrings/helm.gpg] https://baltocdn.com/helm/stable/debian/ all main" | tee /etc/apt/sources.list.d/helm-stable-debian.list \
    && apt-get update \
    && apt-get install -y helm \
    && rm -rf /var/lib/apt/lists/*

# Set up Go environment
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOPATH="/go"
ENV GOBIN="/go/bin"
ENV PATH="${GOBIN}:${PATH}"
ENV GO111MODULE=on
ENV GOPROXY=direct

# Create Go workspace
RUN mkdir -p /go/bin /go/src /go/pkg

# Install Go tools required by the project
RUN go install golang.org/x/tools/cmd/goimports@latest \
    && go install mvdan.cc/gofumpt@latest \
    && go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0 \
    && go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

# Install lychee (link checker)
RUN curl -L https://github.com/lycheeverse/lychee/releases/download/lychee-v0.18.1/lychee-x86_64-unknown-linux-gnu.tar.gz -o lychee.tar.gz \
    && tar -xzf lychee.tar.gz \
    && mv lychee /usr/local/bin/ \
    && rm lychee.tar.gz

# Install markdownlint-cli
RUN npm install -g markdownlint-cli@0.45.0

# Set up Python virtual environment and install requirements
COPY requirements.txt /tmp/requirements.txt
RUN python3 -m venv /workspace/.venv \
    && /workspace/.venv/bin/pip install --upgrade pip \
    && /workspace/.venv/bin/pip install -r /tmp/requirements.txt

# Install AWS CLI (useful for S3 operations)
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" \
    && unzip awscliv2.zip \
    && ./aws/install \
    && rm -rf aws awscliv2.zip

# Install tmux for terminal multiplexing
RUN apt-get update && apt-get install -y tmux && rm -rf /var/lib/apt/lists/*

# Create scripts directory
RUN mkdir -p /workspace/scripts

# Set up shell environment
RUN echo 'export PATH="/usr/local/go/bin:${GOBIN}:${PATH}"' >> /root/.bashrc \
    && echo 'export GOPATH="/go"' >> /root/.bashrc \
    && echo 'export GOBIN="/go/bin"' >> /root/.bashrc \
    && echo 'export GO111MODULE=on' >> /root/.bashrc \
    && echo 'export GOPROXY=direct' >> /root/.bashrc \
    && echo 'alias ll="ls -la"' >> /root/.bashrc \
    && echo 'alias la="ls -la"' >> /root/.bashrc \
    && echo 'cd /workspace' >> /root/.bashrc

# Expose ports for development servers
EXPOSE 8000 2345

# Default command
CMD ["/bin/bash"]
