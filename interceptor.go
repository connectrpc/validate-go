// Copyright 2023 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package validate provides an interceptor implementation for the Connect that integrates 
// with protovalidate to validate incoming protobuf messages against predefined constraints.
// This interceptor ensures adherence to constraints defined on the proto file without the need
// for extra generated code. Used this interceptor to automatically validate request messages
// and enhance the reliability of data communication.
package validate

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/bufbuild/protovalidate-go"
	"google.golang.org/protobuf/proto"
)

// Interceptor implements the connect.Interceptor interface and serves as a crucial
// layer in the communication pipeline, by validating incoming requests and ensuring
// they conform to defined protovalidate preconditions.
//
// Default Behaviors:
//   - Requests are validated for adherence to the defined message structure.
//   - Responses are not validated, focusing validation efforts on incoming data.
//   - Errors are raised for incoming messages that are not protocol buffer messages.
//   - In case of validation errors, an error detail of the type is attached to provide
//     additional context about the validation failure.
//
// It's recommended to use the Interceptor with server-side handlers rather than
// client connections. Placing the Interceptor on handlers ensures that incoming requests
// are thoroughly validated before they are processed, minimizing the risk of handling
// invalid or unexpected data.
type Interceptor struct {
	validator *protovalidate.Validator
}

// Option is a functional option for the Interceptor.
type Option func(*Interceptor)

// NewInterceptor returns a new instance of the Interceptor. It accepts an optional functional
// option to customize its behavior. If no custom validator is provided, a default validator
// is used for message validation.
//
// Usage:
//
//	interceptor, err := NewInterceptor(WithValidator(customValidator))
//	if err != nil {
//	  // Handle error
//	}
//
//	path, handler := examplev1connect.NewExampleServiceHandler(
//		server,
//		connect.WithInterceptors(interceptor),
//	)
func NewInterceptor(opts ...Option) (*Interceptor, error) {
	out := &Interceptor{}
	for _, apply := range opts {
		apply(out)
	}

	if out.validator == nil {
		validator, err := protovalidate.New()
		if err != nil {
			return nil, err
		}
		out.validator = validator
	}

	return out, nil
}

// WithValidator sets the validator to be used for message validation.
// This option allows customization of the validator used by the Interceptor.
func WithValidator(validator *protovalidate.Validator) Option {
	return func(i *Interceptor) {
		i.validator = validator
	}
}

// WrapUnary implements the connect.Interceptor interface.
func (i *Interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if err := validate(i.validator, req.Any()); err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

// WrapStreamingClient implements the connect.Interceptor interface.
func (i *Interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		return &streamingClientInterceptor{
			validator:           i.validator,
			StreamingClientConn: next(ctx, spec),
		}
	}
}

// WrapStreamingHandler implements the connect.Interceptor interface.
func (i *Interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		return next(ctx, &streamingHandlerInterceptor{
			validator:            i.validator,
			StreamingHandlerConn: conn,
		})
	}
}

type streamingClientInterceptor struct {
	connect.StreamingClientConn

	validator *protovalidate.Validator
}

func (s *streamingClientInterceptor) Send(msg any) error {
	if err := validate(s.validator, msg); err != nil {
		return err
	}
	return s.StreamingClientConn.Send(msg)
}

type streamingHandlerInterceptor struct {
	connect.StreamingHandlerConn

	validator *protovalidate.Validator
}

func (s *streamingHandlerInterceptor) Receive(msg any) error {
	if err := s.StreamingHandlerConn.Receive(msg); err != nil {
		return err
	}
	return validate(s.validator, msg)
}

func validate(validator *protovalidate.Validator, msg any) error {
	protoMessage, ok := msg.(proto.Message)
	if !ok {
		return fmt.Errorf("message is not a proto.Message: %T", msg)
	}
	if err := validator.Validate(protoMessage); err != nil {
		out := connect.NewError(connect.CodeInvalidArgument, err)
		var validationErr *protovalidate.ValidationError
		if errors.As(err, &validationErr) {
			if detail, err := connect.NewErrorDetail(validationErr.ToProto()); err == nil {
				out.AddDetail(detail)
			}
		}
		return out
	}
	return nil
}
