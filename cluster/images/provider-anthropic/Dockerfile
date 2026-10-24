# Digest-pinned so Renovate tracks rebuilds of the `nonroot` tag explicitly
# instead of floating on it silently.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:18f4cc26c9ffa7f20a359298be7deadac5c607c8e03a02b8119f1a5fc2f82c99

ARG TARGETOS
ARG TARGETARCH

ENV USER_ID=65532

USER ${USER_ID}
EXPOSE 8080

ADD "bin/${TARGETOS}_${TARGETARCH}/provider" /usr/local/bin/provider

ENTRYPOINT ["provider"]