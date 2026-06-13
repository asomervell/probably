# PostgreSQL 18 with pg_textsearch extension for BM25 search
FROM postgres:18-alpine

# Install build dependencies
RUN apk add --no-cache --virtual .build-deps \
    git \
    make \
    gcc \
    musl-dev \
    clang \
    llvm \
    lld

# Create symlinks for clang-19 and llvm-19 tools (PostgreSQL 18 was built with LLVM 19 for JIT)
RUN ln -sf /usr/bin/clang /usr/bin/clang-19 && \
    ln -sf /usr/bin/clang++ /usr/bin/clang++-19 && \
    mkdir -p /usr/lib/llvm19/bin && \
    ln -sf /usr/bin/llvm-lto /usr/lib/llvm19/bin/llvm-lto

# Clone and build pg_textsearch
RUN cd /tmp && \
    git clone --depth 1 https://github.com/timescale/pg_textsearch.git && \
    cd pg_textsearch && \
    make USE_PGXS=1 && \
    make USE_PGXS=1 install && \
    cd / && \
    rm -rf /tmp/pg_textsearch

# Clean up build dependencies to reduce image size
RUN apk del .build-deps

# The extension will be available to CREATE EXTENSION pg_textsearch;
