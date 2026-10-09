# analytics-flow-repo-v2

Generate swagger docs:

    swag init -g api.go -o docs -dir pkg/api --parseDependency --ot json

swag v1.16.4 reproduces the committed file byte for byte when nothing changed.

## Operator usage

`GET /operators/{id}/usage` is open to every authenticated user and answers `{"flows": n, "readable": [{"id", "name"}]}`. `flows` counts the flows of all users that contain a node of the operator; `readable` lists only those the caller may read, selected like the flow listing. The operator repository asks this before it deletes an operator.

## Deleting a flow that is in use

`DELETE /flow/{id}/` asks two services before it deletes: the pipeline registry (`PIPELINE_REGISTRY_URL`) and the smart-service-repository (`GET /resource-usage/flows/{id}`, with the caller's `Authorization` header).

| Env var | Default | Notes |
|---|---|---|
| `SMART_SERVICE_REPOSITORY_URL` | `http://api.smart-service-repository:8080` | Address of the smart-service-repository. The in-cluster service; no path prefix. Empty is refused at startup. In a config file: `smart_service_repository_url`. |

- Used by pipelines or by smart service releases: 409 with `{"error", "pipelines", "releases", "instances", "readable": [{"id", "design_id", "name"}]}`. The body used to be the plain text `still in use`; `error` carries that text. `pipelines` counts the pipelines of all users without naming them, `releases` and `instances` count over all users, `readable` lists only the smart service designs the caller may read.
- The pipeline registry or the smart-service-repository cannot be asked (network error, any status but 200 or 204, an answer without the counts): nothing is deleted. The registry gives 424, the smart-service-repository 502.
- `?force=true` deletes anyway. It is for administrators only (role `admin` in `X-User-Roles`, or in the token when the header is absent); anyone else gets 403. Both services are still asked, so a forced delete is refused when either cannot answer. A forced delete of a used flow is logged at warn with `flow_id`, `user_id`, `pipelines`, `releases` and `instances`.

The check and the delete are not atomic: a pipeline or release created between them is not seen.

`PERMISSIONS_V2_URL=mock` does not mock the smart-service-repository; deletes need a reachable `SMART_SERVICE_REPOSITORY_URL` there as well.

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
