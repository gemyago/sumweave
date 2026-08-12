# Copied from gemyago/golang-backend-boilerplate@798f0dc9fd753481d0d698d8232ea08df44185b6 and adapted for one prepared binary.
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian12:nonroot
FROM --platform=$TARGETPLATFORM ${RUNTIME_IMAGE}

ARG TARGETOS
ARG TARGETARCH

COPY build/dist/${TARGETOS}/${TARGETARCH}/sumweave /usr/local/bin/sumweave
COPY build/dist/platform-agents/skills /app/.platform-agents/skills

ENTRYPOINT ["sumweave"]
