FROM golang:1.27-trixie@sha256:c65ecfe613b539ac74c62a4c3b2c27e8f8a6076384c8f27e8a3c0f3be17cc385
# The tests have no dependencies. With no proxy and a local toolchain, go never goes to the network.
ENV GOTOOLCHAIN=local GOPROXY=off GOTELEMETRY=off
USER 1000:1000
