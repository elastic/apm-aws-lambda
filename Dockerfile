# Pin to Alpine 3.16.8
# For a complete list of hashes, see:
# https://github.com/docker-library/repo-info/tree/master/repos/alpine/remote
FROM --platform=$BUILDPLATFORM alpine@sha256:e4cdb7d47b06ba0a062ad2a97a7d154967c8f83934594d9f2bd3efa89292996b
ARG EXTENSION_FILE
ARG COMMIT_TIMESTAMP
COPY ${EXTENSION_FILE} /opt/elastic-apm-extension
COPY NOTICE.txt dependencies.asciidoc /opt/

# Related to reproducible builds
RUN find /opt -exec touch -am -d $(date -u -d @"${COMMIT_TIMESTAMP}" "+%Y%m%d%H%M.%S") -t $(date -u -d @"${COMMIT_TIMESTAMP}" "+%Y%m%d%H%M.%S") {} \;

