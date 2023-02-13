# Pin to Alpine 3.16.2
# For a complete list of hashes, see:
# https://github.com/docker-library/repo-info/tree/master/repos/alpine/remote
FROM --platform=$BUILDPLATFORM alpine@sha256:b95359c2505145f16c6aa384f9cc74eeff78eb36d308ca4fd902eeeb0a0b161b
ARG EXTENSION_FILE
ARG COMMIT_TIMESTAMP
COPY ${EXTENSION_FILE} /opt/elastic-apm-extension
COPY NOTICE.txt dependencies.asciidoc /opt/

# Related to reproducible builds
RUN find /opt -exec touch -amdt "${COMMIT_TIMESTAMP}" {} \;

