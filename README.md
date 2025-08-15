# oapi-sqlc

web apis with close to only code generation through oapi-codegen and sqlc

## Features

- **Code Generation**: Automatically generate API client code and database models from OpenAPI specifications and SQL schemas. (docs are reached through `GET /docs`)
- **Database**: Uses pgx for database interactions.

## Development

#### Generating Code

```sh
go generate ./...
```

#### Running Tests

```sh
docker compose -f docker-compose.ci.yml up -d --wait
go test -v -coverpkg=./... -coverprofile cover.out ./...
docker compose -f docker-compose.ci.yml down
```

#### Linting Code

```sh
golangci-lint run
```
