FROM golang:1.24-alpine AS builder
RUN apk add --no-cache build-base git

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

ENV CGO_ENABLED=0

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -o /usr/local/bin/rbln-npu-feature-discovery ./cmd/rbln-npu-feature-discovery

FROM redhat/ubi9-minimal:9.6
ARG VERSION

RUN microdnf install -y shadow-utils && \
    groupadd -r rbln && \
    useradd -r -g rbln -d /home/rbln -s /sbin/nologin -c "RBLN NPU Feature Discovery" rbln && \
    mkdir -p /home/rbln && \
    chown rbln:rbln /home/rbln && \
    microdnf clean all

COPY LICENSE /licenses/LICENSE.txt

LABEL \
    name="rbln-npu-feature-discovery" \
    vendor="Rebellions" \
    version="${VERSION}" \
    release="N/A" \
    summary="Rebellions NPU Feature Discovery" \
    description="NPU Feature Discovery extends Kubernetes Node Feature Discovery to automatically detect Rebellions NPUs on a node and generate the corresponding Kubernetes labels." \
    maintainer="Rebellions sw_devops@rebellions.ai" \
    io.k8s.display-name="Rebellions NPU Feature Discovery" \
    com.redhat.component="rbln-npu-feature-discovery"

COPY --from=builder /usr/local/bin/rbln-npu-feature-discovery /usr/local/bin/rbln-npu-feature-discovery
RUN chown rbln:rbln /usr/local/bin/rbln-npu-feature-discovery && \
    chmod 755 /usr/local/bin/rbln-npu-feature-discovery

USER rbln

ENV RBLN_NPU_FEATURE_DISCOVERY_LOG_LEVEL=info

ENTRYPOINT ["/usr/local/bin/rbln-npu-feature-discovery"]
