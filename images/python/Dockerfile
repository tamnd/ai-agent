FROM python:3.14-slim-trixie@sha256:930e1abba7c4e8f96ed93e8012c35d174d7732752731bbd5e8e757d7ec6f3966
COPY requirements.txt /opt/aik/requirements.txt
RUN pip install --no-cache-dir --no-deps --require-hashes --only-binary=:all: -r /opt/aik/requirements.txt
ENV PYTHONDONTWRITEBYTECODE=1
USER 1000:1000
