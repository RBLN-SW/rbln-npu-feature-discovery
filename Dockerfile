ARG GOLANG_VERSION=1.26.10
FROM --platform=$BUILDPLATFORM golang:${GOLANG_VERSION}-alpine AS builder
RUN apk add --no-cache build-base git

WORKDIR /src

COPY . .

ENV CGO_ENABLED=0

# Set by buildx per target platform. Cross-compiling on the build host keeps the
# Go toolchain off QEMU when the image is built for arm64.
ARG TARGETOS TARGETARCH

ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -mod=vendor -buildvcs=false \
    -ldflags "-X github.com/rebellions-sw/rbln-npu-feature-discovery/internal/cmd.version=${VERSION}" \
    -o /usr/local/bin/rbln-npu-feature-discovery ./cmd/rbln-npu-feature-discovery

# Install runtime RPMs in a UBI Micro filesystem, keeping the package
# manager and its networking dependencies in the build stage.
FROM redhat/ubi9-micro:9.9 AS runtime-base
FROM redhat/ubi9:9.9 AS runtime-rootfs
COPY --from=runtime-base / /mnt/rootfs/
RUN dnf -y --installroot=/mnt/rootfs --releasever=9 \
        --setopt=install_weak_deps=False --setopt=tsflags=nodocs \
        install ca-certificates && \
    dnf --installroot=/mnt/rootfs clean all
RUN groupadd --prefix /mnt/rootfs -r -g 999 rbln && \
    useradd --prefix /mnt/rootfs -r -u 999 -g 999 -M -d /home/rbln -s /sbin/nologin -c "RBLN NPU Feature Discovery" rbln && \
    install -d -o 999 -g 999 /mnt/rootfs/home/rbln

FROM runtime-base
COPY --from=runtime-rootfs /mnt/rootfs/ /
ARG VERSION

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
# Bundled Rebellions pci.ids block: product-name fallback for hosts whose
# driver predates the card_name sysfs attribute. First path probed by
# internal/pciids.FindPCIIDsPath.
COPY deps/rebellions-pci.ids /usr/share/misc/pci.ids
RUN chown rbln:rbln /usr/local/bin/rbln-npu-feature-discovery && \
    chmod 755 /usr/local/bin/rbln-npu-feature-discovery

USER rbln

ENV RBLN_NPU_FEATURE_DISCOVERY_LOG_LEVEL=info \
    RBLN_NPU_FEATURE_DISCOVERY_LOG_FORMAT=json

ENTRYPOINT ["/usr/local/bin/rbln-npu-feature-discovery"]
