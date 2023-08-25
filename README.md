# ConnectRPC Validation Interceptor

[![Build](https://github.com/connectrpc/validate-go/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/connectrpc/validate-go/actions/workflows/ci.yaml)
[![Report Card](https://goreportcard.com/badge/connectrpc.com/validate)](https://goreportcard.com/report/connectrpc.com/validate)
[![GoDoc](https://pkg.go.dev/badge/connectrpc.com/validate.svg)](https://pkg.go.dev/connectrpc.com/validate)

The `validate` package provides an interceptor implementation for the ConnectRPC
framework. It integrates with the [`protovalidate-go`][protovalidate-go] library
to validate incoming protobuf messages, ensuring adherence to the defined
message structure. This interceptor is a crucial layer in the communication
pipeline, enhancing data integrity and reliability within the ConnectRPC
framework.

## Installation

To use the `validate` package, you need to have Go installed. You can then
install the package using:

```sh
go get -u connectrpc.com/validate
```

## Usage

To use the `Interceptor`, follow these steps:

1. Import the necessary packages:

    ```go
    import (
        "context"
        
        "connectrpc.com/connect"
        "connectrpc.com/validate"
    )
    ```

2. Create a custom validator if needed (optional):

    ```go
    validator := protovalidate.New() // Customize the validator as needed
    ```

   > See [`protovalidate`][protovalidate] for more information on how to
   construct
   > a validator.

3. Create an instance of the `Interceptor` using `NewInterceptor`:

    ```go
    interceptor, err := validate.NewInterceptor(validate.WithInterceptor(validator))
    if err != nil {
        // Handle error
    }
    ```

   > If you do not provide a custom validator, the interceptor will create and
   use
   > a default validator.

4. Apply the interceptor to your ConnectRPC server's handlers:

    ```go
    path, handler := examplev1connect.NewExampleServiceHandler(
        server,
        connect.WithInterceptors(interceptor),
    )
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
[protovalidate]: https://github.com/bufbuild/protovalidate
