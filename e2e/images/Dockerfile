# One image for each tmux target of the end-to-end tests. Build a target with
# --target <name>. The images hold tmux and the shells only: the runner mounts
# the glaze binary, the test binary and the e2e module at run time.
#
# Each base is pinned by digest. Dependabot updates the digests and never moves
# a tag, because each tag is a test target.

FROM debian:bookworm-slim@sha256:734a72b3d6142a71993b4bc580ca01d8ca687f77a2c422d96a961d3376b179bc AS debian-bookworm
FROM debian:trixie-slim@sha256:a72b4433de1cf7d6bae0c4cee74d0547f58601a643b3375f070c51600d63b6b2 AS debian-trixie
FROM ubuntu:22.04@sha256:076e7bf82319619f28dd0aa9c02ab34b055ad37100395b4f49a2ffba04fb5e11 AS ubuntu-jammy
FROM ubuntu:24.04@sha256:fe81560990fae86dc4743202b2cb4d3df98d0bcf949173cecef15ce1127ef852 AS ubuntu-noble
FROM alpine:3.22@sha256:2e0b81d60597f449baa56dc25d355367869d2dca4cd57e866f1e2b52af3eb55f AS alpine-322

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
