FROM golang:latest AS builder

RUN mkdir -p /app
COPY . /app/passd
WORKDIR /app/passd
RUN go build ./cmd/passd.go

# NB: I tried to use alpine here but I would get "exec /app/passd: no such file or directory" when attempting
# to run the exe. The same would be true when running the container directly & invoking it, despite the fact that
# the file was discoverable by ls. Not sure why but that doesn't happen on ubuntu.
FROM ubuntu:latest
ARG GIT_SHA

# Need to install ca-certificates explicitly to get the LetsEncrypt root cert
RUN apt update
RUN apt install ca-certificates -y
RUN mkdir -p /app
RUN mkdir -p /app/data
COPY --from=builder /app/passd/passd /app/passd
COPY --from=builder /app/passd/assets /app/assets

ENV PASSD_PORT=3333
ENV PASSD_DB_PATH=/app/data/passd.sqlite
ENV PASSD_KEY_PATH=/app/data/passd.key
ENV PASSD_STATIC_FILES_DIR=/app/assets
ENV PASSD_AUTH_PROVIDER_URL=http://localhost:8080/realms/myrealm
ENV PASSD_REDIRECT_URL=http://localhost:3333/notes/auth/callback

LABEL dev.quemot.passd.image.sha=$GIT_SHA

ENTRYPOINT [ "/app/passd" ]