# oapi-sqlc

web apis with close to only code generation through oapi-codegen and sqlc

## Development

#### Generating Code

```sh
go generate ./...
```

#### Running Tests

```sh
docker compose up -d --wait
go test -v ./...
docker compose down
```

#### Linting Code

```sh
golangci-lint run
```
