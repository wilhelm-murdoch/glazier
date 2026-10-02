# Test image for the glaze release binary on a Debian or Ubuntu base.
# BASE names one of the stages below; each base is pinned by digest, which Dependabot keeps current.
ARG BASE=bookworm
FROM debian:bookworm-slim@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251 AS bookworm
FROM debian:trixie-slim@sha256:a99cfc517144bc59b1978475ec53b46ecabec7e43635402ee5b77cc54cd1b20a AS trixie
FROM ubuntu:22.04@sha256:b8b6ee6aa931ecd9d0d952abc34dc0e5f7c6a30c6bb71b079fe399fde0329c02 AS jammy
FROM ubuntu:24.04@sha256:008173c23f95b170204355c12626cb5a965d779a7e1283b09e9cffbb1bf33ca3 AS noble

FROM ${BASE}
ARG GLAZE_VERSION=v0.1.6
ARG TARGETARCH
RUN apt-get update \
 && apt-get install -y --no-install-recommends tmux curl unzip ca-certificates procps bash coreutils bsdutils zsh fish dash \
 && rm -rf /var/lib/apt/lists/*
ENV LANG=C.UTF-8 LC_ALL=C.UTF-8
WORKDIR /opt/glaze
RUN curl -fsSLO https://github.com/wilhelm-murdoch/glazier/releases/download/${GLAZE_VERSION}/glaze-linux-${TARGETARCH}.zip \
 && curl -fsSLO https://github.com/wilhelm-murdoch/glazier/releases/download/${GLAZE_VERSION}/SHA256SUMS \
 && sha256sum -c SHA256SUMS --ignore-missing \
 && unzip -o glaze-linux-${TARGETARCH}.zip
