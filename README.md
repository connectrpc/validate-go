# Validate

[![Build](https://github.com/connectrpc/validate-go/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/connectrpc/validate-go/actions/workflows/ci.yaml)
[![Report Card](https://goreportcard.com/badge/connectrpc.com/validate)](https://goreportcard.com/report/connectrpc.com/validate)
[![GoDoc](https://pkg.go.dev/badge/connectrpc.com/validate.svg)](https://pkg.go.dev/connectrpc.com/validate)

`connectrpc.com/validate` provides a [Connect][connect-go] interceptor that
takes the tedium out of data validation. Rather than hand-writing repetitive
documentation and code &mdash; verifying that `User.email` is valid, or that
`User.age` falls within reasonable bounds &mdash; you can instead encode those
constraints into your Protobuf schemas and automatically enforce them at
runtime.

Under the hood, this package is powered by [protovalidate][protovalidate-go]
and the [Common Expression Language][cel-spec]. Together, they make validation
flexible, efficient, and consistent across languages _without_ additional code
generation.

## Installation

```bash
go get connectrpc.com/validate
```

## A small example

Curious what all this looks like in practice? First, let's define a schema for
our user service:

```protobuf
syntax = "proto3";

package example.user.v1;

import "buf/validate/validate.proto";
import "google/protobuf/timestamp.proto";

message User {
  // Simple constraints, like checking that an email address is valid, are
  // predefined.
  string email = 1 [(buf.validate.field).string.email = true];

  // For more complex use cases, like comparing fields against each other, we
  // can write a CEL expression.
  google.protobuf.Timestamp birth_date = 2;
  google.protobuf.Timestamp signup_date = 3;

  option (buf.validate.message).cel = {
    id: "user.signup_date",
    message: "signup date must be on or after birth date",
    expression: "this.signup_date >= this.birth_date"
  };
}

message CreateUserRequest {
  User user = 1;
}

message CreateUserResponse {
  User user = 1;
}

service UserService {
  rpc CreateUser(CreateUserRequest) returns (CreateUserResponse) {}
}
```

Notice that simple constraints, like checking email addresses, are short and
declarative. When we need a more elaborate constraint, we can write a custom
CEL expression, customize the error message, and much more. (See [the
main protovalidate repository][protovalidate] for more examples.)

After implementing `UserService`, we can add a validating interceptor with just
one option:


```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	userv1 "connectrpc.com/validate/internal/gen/example/user/v1"
	"connectrpc.com/validate/internal/gen/validate/example/v1/userv1connect"
)

func main() {
	mux := http.NewServeMux()
	mux.Handle(userv1connect.NewUserServiceHandler(
		&userv1connect.UnimplementedUserServiceHandler{},
		connect.WithInterceptors(validate.NewInterceptor()),
	))

	http.ListenAndServe("localhost:8080", mux)
}
```

With the `validate.Interceptor` applied, our `UserService` implementation can
assume that all requests (and optionally responses) have already been
validated &mdash; no need for hand-written boilerplate!

## FAQ

### Does this interceptor work with Connect clients?

Yes: it validates request messages before sending them to the server, and optionally
responses when they are received. But unless you're _sure_ that your clients always have
an up-to-date schema, it's better to let the server handle validation.

### How do clients know which fields are invalid?

If the message fails validation, the interceptor returns an error coded
with `connect.CodeInvalidArgument`. It also adds a [detailed representation of the
validation error(s)][violations] as an [error detail][connect-error-detail].

### How should schemas import protovalidate's options?

Because this interceptor uses [protovalidate][protovalidate-go], it doesn't
need any generated code for validation. However, any Protobuf schemas with
constraints must import [`buf/validate/validate.proto`][validate.proto]. It's
easiest to import this file directly from the [Buf Schema
Registry][bsr]: this repository contains an [example
schema](internal/proto/example/user/v1/user.proto) with constraints,
[buf.yaml](internal/proto/buf.yaml) and [buf.gen.yaml](buf.gen.yaml)
configuration files, and `make generate` [recipe](Makefile).

### Does the interceptor validate responses?

By default, on both clients and servers, the interceptor only validates requests.
If you'd additionally like to validate responses, use the `WithValidateResponses`
option when constructing your `Interceptor`.

## Ecosystem

* [connect-go]: the Connect runtime
* [protovalidate-go]: the underlying Protobuf validation library
* [protovalidate]: schemas and documentation for the constraint language
* [CEL][cel-spec]: the Common Expression Language

## Status: Unstable

This module is unstable. Expect breaking changes as we iterate toward a stable
release.

It supports:

* The two most recent major releases of Go. Keep in mind that [only the last
  two releases receive security patches][go-support-policy].
* [APIv2] of Protocol Buffers in Go (`google.golang.org/protobuf`).

Within those parameters, this project follows semantic versioning. Once we tag
a stable release, we will _not_ make breaking changes without incrementing the
major version.


## License

Offered under the [Apache 2 license](LICENSE).

[APIv2]: https://blog.golang.org/protobuf-apiv2
[bsr]: https://buf.build
[cel-spec]: https://github.com/google/cel-spec
[connect-error-detail]: https://pkg.go.dev/connectrpc.com/connect#ErrorDetail
[connect-go]: https://github.com/connectrpc/connect-go
[go-support-policy]: https://golang.org/doc/devel/release#policy
[protovalidate-go]: https://github.com/bufbuild/protovalidate-go
[protovalidate]: https://github.com/bufbuild/protovalidate
[validate.proto]: https://github.com/bufbuild/protovalidate/blob/main/proto/protovalidate/buf/validate/validate.proto
[violations]: https://pkg.go.dev/buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate#Violations
