# Pin to Alpine 3.9
# For a complete list of hashes, see:
# https://github.com/docker-library/repo-info/tree/master/repos/alpine/remote
FROM alpine@sha256:115731bab0862031b44766733890091c17924f9b7781b79997f5f163be262178
ARG EXTENSION_FILE
COPY ${EXTENSION_FILE} /opt/elastic-apm-extension
COPY /NOTICE.txt /opt/NOTICE.txt
COPY /dependencies.asciidoc /opt/dependencies.asciidoc
