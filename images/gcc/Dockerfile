FROM gcc:15-trixie@sha256:b803fd6fd7ebde53965c3e9bd9ce5403932e9fca382467b190c2bb41528eb3d8
# The recipes call cc and c++.
RUN ln -sf "$(command -v gcc)" /usr/local/bin/cc && ln -sf "$(command -v g++)" /usr/local/bin/c++
USER 1000:1000
