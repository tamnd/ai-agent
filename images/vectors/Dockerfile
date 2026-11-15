# The image of the official metric implementations, aik-vectors (spec 08.7.5 and 08.9.1). It runs only aik vectors make, which writes the test vectors of the metric nodes with these packages, and never the code of impl/: the official packages never enter the reference sets. Each package is pinned by version and hash, and pip installs wheels only.
FROM python:3.14-slim-trixie@sha256:930e1abba7c4e8f96ed93e8012c35d174d7732752731bbd5e8e757d7ec6f3966
COPY requirements.txt /opt/aik/requirements.txt
RUN pip install --no-cache-dir --no-deps --require-hashes --only-binary=:all: -r /opt/aik/requirements.txt
ENV PYTHONDONTWRITEBYTECODE=1 OMP_NUM_THREADS=1 MKL_NUM_THREADS=1
USER 1000:1000
