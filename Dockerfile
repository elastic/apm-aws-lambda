# Pin to Alpine 3.16.2
# For a complete list of hashes, see:
# https://github.com/docker-library/repo-info/tree/master/repos/alpine/remote
FROM alpine@sha256:bc41182d7ef5ffc53a40b044e725193bc10142a1243f395ee852a8d9730fc2ad
ARG EXTENSION_FILE
ARG BUILD_DATE
COPY ${EXTENSION_FILE} /opt/elastic-apm-extension
COPY NOTICE.txt dependencies.asciidoc /opt/

# Related to reproducible builds
RUN find /opt -exec touch -t "${BUILD_TIME}" {} \;

