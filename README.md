`go install github.com/frroossst/pls-dont-go/cmd/immutablelint@v0.9.0`

```
git clone https://github.com/frroossst/pls-dont-go.git && cd pls-dont-go
make help
```

---


`make build` to build a immutablelint binary in the current folder.

enable logging by `immutablelint --log=stderr` to print to stderr or `immutablelint --log=myFile.log` to log to a file

`make test` to run tests. Change `examples/all.go` to add more test cases.

`make lint` 
1. installs golangci-lint using `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`
2. builds a custom-gcl binary with the immutablecheck plugin
3. runs the custom-gcl on the `examples` folder using `./custom-gcl run ./examples/...` 
