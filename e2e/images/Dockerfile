# One image for each tmux target of the end-to-end tests. Build a target with
# --target <name>. The images hold tmux and the shells only: the runner mounts
# the glaze binary, the test binary and the e2e module at run time.
#
# Each base is pinned by digest. Dependabot updates the digests and never moves
# a tag, because each tag is a test target.

FROM debian:bookworm-slim@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251 AS debian-bookworm
FROM debian:trixie-slim@sha256:a99cfc517144bc59b1978475ec53b46ecabec7e43635402ee5b77cc54cd1b20a AS debian-trixie
FROM ubuntu:22.04@sha256:b8b6ee6aa931ecd9d0d952abc34dc0e5f7c6a30c6bb71b079fe399fde0329c02 AS ubuntu-jammy
FROM ubuntu:24.04@sha256:008173c23f95b170204355c12626cb5a965d779a7e1283b09e9cffbb1bf33ca3 AS ubuntu-noble
FROM alpine:3.22@sha256:5291449c3df73caf6ed85e649dec1b9e818b39a5d8c871e97afc13e9cd5e8fa8 AS alpine-322

# tmux from the distribution, with bash, zsh, fish and dash for the command
# delivery cases and procps for the process checks.
FROM debian-bookworm AS bookworm
RUN apt-get update \
 && apt-get install -y --no-install-recommends tmux bash zsh fish dash procps \
 && rm -rf /var/lib/apt/lists/*
ENV LANG=C.UTF-8 LC_ALL=C.UTF-8

FROM debian-trixie AS trixie
RUN apt-get update \
 && apt-get install -y --no-install-recommends tmux bash zsh fish dash procps \
 && rm -rf /var/lib/apt/lists/*
ENV LANG=C.UTF-8 LC_ALL=C.UTF-8

FROM ubuntu-jammy AS jammy
RUN apt-get update \
 && apt-get install -y --no-install-recommends tmux bash zsh fish dash procps \
 && rm -rf /var/lib/apt/lists/*
ENV LANG=C.UTF-8 LC_ALL=C.UTF-8

FROM ubuntu-noble AS noble
RUN apt-get update \
 && apt-get install -y --no-install-recommends tmux bash zsh fish dash procps \
 && rm -rf /var/lib/apt/lists/*
ENV LANG=C.UTF-8 LC_ALL=C.UTF-8

FROM alpine-322 AS alpine
RUN apk add --no-cache tmux bash zsh fish dash procps
ENV LANG=C.UTF-8 LC_ALL=C.UTF-8

# tmux from a release tarball, for a version that no distribution ships yet.
# The checksum stops the build when the tarball changes.
FROM debian-bookworm AS tmux-source
ARG TMUX_VERSION=3.7c
ARG TMUX_SHA256=7c60cae9a0e25288e2e24750aafc9e8800fc7fd4555e447e1b29ee4201cfb3bf
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl build-essential pkg-config bison libevent-dev libncurses-dev \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /src
RUN curl -fsSLo tmux.tar.gz "https://github.com/tmux/tmux/releases/download/${TMUX_VERSION}/tmux-${TMUX_VERSION}.tar.gz" \
 && echo "${TMUX_SHA256}  tmux.tar.gz" | sha256sum -c - \
 && tar -xzf tmux.tar.gz --strip-components=1 \
 && ./configure --prefix=/usr/local >/dev/null \
 && make -j"$(nproc)" >/dev/null \
 && make install >/dev/null

FROM debian-bookworm AS tmux37
RUN apt-get update \
 && apt-get install -y --no-install-recommends libevent-2.1-7 libevent-core-2.1-7 libtinfo6 libncursesw6 bash zsh fish dash procps \
 && rm -rf /var/lib/apt/lists/*
COPY --from=tmux-source /usr/local/bin/tmux /usr/local/bin/tmux
ENV LANG=C.UTF-8 LC_ALL=C.UTF-8

# tmux from a commit of its development branch, for the weekly canary. The
# runner resolves the branch to a commit, so a new commit rebuilds the image.
# This target is not in the default set: a change in tmux must not block glaze.
FROM debian-bookworm AS tmux-next-source
ARG TMUX_COMMIT
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates git build-essential pkg-config bison autoconf automake libevent-dev libncurses-dev \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /src
RUN test -n "${TMUX_COMMIT}" \
 && git init -q . \
 && git fetch -q --depth 1 https://github.com/tmux/tmux.git "${TMUX_COMMIT}" \
 && git checkout -q FETCH_HEAD \
 && sh autogen.sh >/dev/null \
 && ./configure --prefix=/usr/local >/dev/null \
 && make -j"$(nproc)" >/dev/null \
 && make install >/dev/null

FROM debian-bookworm AS tmuxnext
RUN apt-get update \
 && apt-get install -y --no-install-recommends libevent-2.1-7 libevent-core-2.1-7 libtinfo6 libncursesw6 bash zsh fish dash procps \
 && rm -rf /var/lib/apt/lists/*
COPY --from=tmux-next-source /usr/local/bin/tmux /usr/local/bin/tmux
ENV LANG=C.UTF-8 LC_ALL=C.UTF-8
