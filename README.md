`go install github.com/frroossst/pls-dont-go/cmd/immutablelint@latest`

```
git clone https://github.com/frroossst/pls-dont-go.git && cd pls-dont-go
make help
```

`make build` to build a immutablelint binary in the current folder.

enable logging by remove the early return in `immutablecheck/logger.go:38` and recompile using `make build`.

`make test` to run tests. Change `examples/all.go` to add more test cases.

`make lint` 
1. installs golangci-lint using `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
2. builds a custom-gcl binary with the immutablecheck plugin
3. runs the custom-gcl on the `examples` folder using `./custom-gcl run ./examples/...` 
