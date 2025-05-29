# syntax=docker/dockerfile:1.4

FROM rust:1.84-alpine AS builder
RUN apk add --no-cache build-base protobuf-dev
WORKDIR /app
COPY Cargo.toml Cargo.lock ./
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/app/target \
    cargo fetch
COPY proto ./proto
COPY src ./src
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/app/target \
    cargo build --release --locked --bin rbln-npu-feature-discovery && \
    cp /app/target/release/rbln-npu-feature-discovery /usr/local/bin/

FROM alpine:3.18
COPY --from=builder /usr/local/bin/rbln-npu-feature-discovery /usr/bin/rbln-npu-feature-discovery
ENV RUST_LOG=info
ENTRYPOINT ["/usr/bin/rbln-npu-feature-discovery"]
