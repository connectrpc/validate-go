# ConnectRPC Validate Go

This repository contains a Go package named `validate` that provides an
interceptor implementation for the ConnectRPC framework. The interceptor is
designed to perform protocol buffer message validation using
the `protovalidate-go` library. The provided interceptor can be used to ensure
that incoming requests and messages adhere to the defined protocol buffer
message structure.

## Installation

To use the `validate` package in your Go project, you can add it as a dependency
using `go get`:

```bash
go get connectrpc.com/validate
```

## Usage

The `validate` package offers an interceptor named `Interceptor` that implements
the `connect.Interceptor` interface. This interceptor is used to validate
incoming messages using the `protovalidate-go` library before passing them on to
the next interceptor or handler.

### Creating an Interceptor

To create a new `Interceptor`, you can use the `NewInterceptor` function
provided by the package:

```go
validator := protovalidate.NewValidator() // Initialize your protovalidate validator
interceptor := validate.NewInterceptor(validator)
```

### Wrapping Unary Functions

You can wrap a `connect.UnaryFunc` with the interceptor's validation using
the `WrapUnary` method. This ensures that the incoming request is validated
before being processed:

```go
unaryFunc := func (ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
// Your unary function logic
}

wrappedUnaryFunc := interceptor.WrapUnary(unaryFunc)
```

### Wrapping Streaming Clients

For streaming clients, you can wrap a `connect.StreamingClientFunc` with the
interceptor's validation using the `WrapStreamingClient` method:

```go
streamingClientFunc := func (ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
// Your streaming client logic
}

wrappedStreamingClientFunc := interceptor.WrapStreamingClient(streamingClientFunc)
```

### Wrapping Streaming Handlers

When dealing with streaming handlers, you can wrap
a `connect.StreamingHandlerFunc` with the interceptor's validation using
the `WrapStreamingHandler` method:

```go
streamingHandlerFunc := func (ctx context.Context, conn connect.StreamingHandlerConn) error {
// Your streaming handler logic
}

wrappedStreamingHandlerFunc := interceptor.WrapStreamingHandler(streamingHandlerFunc)
```

## License

This package is distributed under the [MIT License](LICENSE).

