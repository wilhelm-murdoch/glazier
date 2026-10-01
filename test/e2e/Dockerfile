# Test image for the glaze release binary on a Debian or Ubuntu base.
ARG BASE=debian:bookworm-slim
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
