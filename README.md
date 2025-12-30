# Simple Password Service (`passd`)

This is a simple key-value store with values encrypted with a key stored locally on the service. This is not intended to be something capital-S Secure in the sense that you would want to use it for an authorization provider, but it should be basically sufficient to provide simple one-off passwords (e.g. sites that contain contact info, like birthday invites or wedding pages).

Passwords are encrypted with AES-GCM &amp; stored alongside an arbitrary string ID in a SQLite database. Password CRUD is done via an authenticated API, whereas password validation is unauthenticated (for obvious reasons).

This setup does not really protect you from someone getting into your machine, but does at least ensure that data extrication via something like SQL injection will make password recovery difficult.

For API call examples, see [passd.http](./passd.http).

## Building

This is a standard Go application, and can be built with standard Go commands. Ensure that `gcc` is installed locally so that `CGO_ENABLED` is picked up (necessary for the `sqlite3` driver).

    go build ./cmd/passd.go

    # OR

    make compile

Compiling the image is similarly straightforward, just run the `build-image` target:

    make build-image

## Running locally

Use the local [Docker Compose project](./docker-compose.yml). It relies on having a locally-built [`quemot-dev/auth` image](https://github.com/mrshanahan/quemot-dev-auth).

You can disable auth by passing the `DISABLE_AUTH` environment variable when invoking `docker compose`. You can similarly control the `API_PORT` this way.

    # with auth
    docker compose up -d

    # without auth
    DISABLE_AUTH=1 docker compose up -d

By default the app will be serving requests on `http://localhost:5555`.

