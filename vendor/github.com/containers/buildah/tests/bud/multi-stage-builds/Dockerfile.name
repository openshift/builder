FROM alpine AS myname
COPY Dockerfile.name /

FROM scratch
COPY --from=myname /Dockerfile.name /Dockerfile.name
