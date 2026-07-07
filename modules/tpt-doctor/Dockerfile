# Dockerfile for tpt-doctor. Build from the repository root:
#   docker build -f modules/tpt-doctor/Dockerfile -t tpt-doctor .
# (the build context must be the repo root so ../../core resolves via the
# module's replace directive).

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY core ./core
COPY modules/tpt-doctor ./modules/tpt-doctor
WORKDIR /src/modules/tpt-doctor
RUN go build -o /out/tpt-doctor ./cmd/tpt-doctor

FROM alpine:3.20
RUN apk add --no-cache ca-certificates postgresql-client
COPY --from=build /out/tpt-doctor /usr/local/bin/tpt-doctor
COPY modules/tpt-doctor/docker-entrypoint.sh /docker-entrypoint.sh
COPY deploy/seed/e2e_seed.sql /seed/e2e_seed.sql
RUN chmod +x /docker-entrypoint.sh
ENTRYPOINT ["/docker-entrypoint.sh"]
