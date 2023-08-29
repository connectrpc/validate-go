# ConnectRPC Validation Interceptor

[![Build](https://github.com/connectrpc/validate-go/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/connectrpc/validate-go/actions/workflows/ci.yaml)
[![Report Card](https://goreportcard.com/badge/connectrpc.com/validate)](https://goreportcard.com/report/connectrpc.com/validate)
[![GoDoc](https://pkg.go.dev/badge/connectrpc.com/validate.svg)](https://pkg.go.dev/connectrpc.com/validate)

`connectrpc.com/validate` adds support for a protovalidate interceptor to Connect servers.

[`protovalidate`][protovalidate-go] is a series of libraries designed to validate Protobuf messages at
runtime based on user-defined validation rules. Powered by Google's Common
Expression Language ([CEL][cel-spec]), it provides a
flexible and efficient foundation for defining and evaluating custom validation
rules. The primary goal of [`protovalidate`][protovalidate] is to help developers ensure data
consistency and integrity across the network without requiring generated code.

## Installation

Add the interceptor to your project with `go get`:

```bash
go get connectrpc.com/validate
```

## An Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	// Generated from your protobuf schema by protoc-gen-go and
	// protoc-gen-connect-go.
	pingv1 "connectrpc.com/validate/internal/gen/connect/ping/v1"
	"connectrpc.com/validate/internal/gen/connect/ping/v1/pingv1connect"
)

func main() {
	interceptor, err := validate.NewInterceptor()
	if err != nil {
		log.Fatal(err)
	}
	
	mux := http.NewServeMux()
	mux.Handle(pingv1connect.NewPingServiceHandler(
		&pingv1connect.UnimplementedPingServiceHandler{},
		connect.WithInterceptors(interceptor),
	))

	http.ListenAndServe("localhost:8080", mux)
}

func makeRequest() {
	client := pingv1connect.NewPingServiceClient(
		http.DefaultClient,
		"http://localhost:8080",
	)
	resp, err := client.Ping(
		context.Background(),
		connect.NewRequest(&pingv1.PingRequest{}),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp)
}
```

By applying the interceptor to your server's handlers, you ensure that incoming
requests are thoroughly validated before being processed. This practice
minimizes the risk of handling invalid or unexpected data, contributing to more
robust and reliable data processing logic.

## Ecosystem

- [connect-go]: The ConnectRPC framework for Go.
- [protovalidate-go]: A protocol buffer message validator for Go.

## License

Offered under the [Apache 2 license](LICENSE).

[connect-go]: https://github.com/connectrpc/connect-go
[protovalidate-go]: https://github.com/bufbuild/protovalidate-go
[cel-spec]: https://github.com/google/cel-spec
[protovalidate]: https://github.com/bufbuild/protovalidate
