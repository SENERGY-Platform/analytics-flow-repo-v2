# analytics-flow-repo-v2

Generate swagger docs:

    swag init -g api.go -o docs -dir pkg/api --parseDependency --ot json

## MongoDB configuration

| Env var | Default | Notes |
|---|---|---|
| `MONGO_URL` | `mongodb://localhost:27017` | Full connection string including scheme, passed to the driver unchanged; must not contain credentials. |
| `MONGO_USER` | empty | No authentication when empty. |
| `MONGO_PASSWORD` | empty | Required when `MONGO_USER` is set; masked in the logged config. |
| `MONGO_AUTH_SOURCE` | `admin` | Database the user is defined in. |
| `MONGO_DATABASE` | `analytics_flow_repo` | Must not be empty; collection is `flows`. |

In a config file these are `mongo_url`, `mongo_user`, `mongo_password`, `mongo_auth_source` and `mongo_database`.

When `MONGO_USER` is set, the credentials are built from `MONGO_USER`, `MONGO_PASSWORD` and `MONGO_AUTH_SOURCE` alone: they replace any user, password, `authSource` and `authMechanism` given in `MONGO_URL`.

The config is logged as JSON at startup, so credentials belong in `MONGO_USER`/`MONGO_PASSWORD`, never in `MONGO_URL`. Startup fails unless an authenticated `listCollections` on `MONGO_DATABASE` succeeds within 10 seconds.

## Tests

    go test ./...

`TestInitDBAuthenticates` runs only without `-short` and when `MONGO_AUTH_TEST_URL`, `MONGO_AUTH_TEST_USER` and `MONGO_AUTH_TEST_PASSWORD` are set. The user and password are root credentials of a throwaway server with access control; the test creates and removes its own users and databases there. Example:

    docker run -d --rm --name flow-auth-test -p 127.0.0.1:27018:27017 -e MONGO_INITDB_ROOT_USERNAME=root -e MONGO_INITDB_ROOT_PASSWORD=rootpw mongo:8.2
    MONGO_AUTH_TEST_URL=mongodb://127.0.0.1:27018 MONGO_AUTH_TEST_USER=root MONGO_AUTH_TEST_PASSWORD=rootpw go test ./pkg/repo/
    docker stop flow-auth-test
