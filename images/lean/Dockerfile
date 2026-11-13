FROM debian:trixie-slim@sha256:f6b08b09a05725f9431502d3b5714f5c4576e82c1a84fa1debe0c24a7e0c3ec4 AS fetch
RUN apt-get update && apt-get install -y --no-install-recommends zstd && rm -rf /var/lib/apt/lists/*
ADD --checksum=sha256:7289597a44543246c9227b8cae501767e095ce17f6d448b95fc4a8dc97268f6d https://github.com/leanprover/lean4/releases/download/v4.35.0/lean-4.35.0-linux.tar.zst /tmp/lean.tar.zst
RUN mkdir /opt/lean && tar --zstd -xf /tmp/lean.tar.zst -C /opt/lean --strip-components=1

FROM debian:trixie-slim@sha256:f6b08b09a05725f9431502d3b5714f5c4576e82c1a84fa1debe0c24a7e0c3ec4
COPY --from=fetch /opt/lean /opt/lean
ENV PATH=/opt/lean/bin:$PATH
USER 1000:1000
