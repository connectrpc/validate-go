// Copyright 2023-2025 The Connect Authors
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

// Package validate provides a [connect.Interceptor] that validates messages
// against constraints specified in their Protobuf schemas. Because the
// interceptor is powered by [protovalidate], validation is flexible,
// efficient, and consistent across languages - without additional code
// generation.
package validate

import (
	"context"
	"errors"
	"fmt"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
)

// An Option configures an [Interceptor].
type Option interface {
	apply(*Interceptor)
}

// WithValidator configures the [Interceptor] to use a customized
// [protovalidate.Validator]. By default, [protovalidate.GlobalInterceptor]
// is used See [protovalidate.ValidatorOption] for the range of available
// customizations.
func WithValidator(validator protovalidate.Validator) Option {
	return optionFunc(func(i *Interceptor) {
		i.validator = validator
	})
}

// WithValidateResponses configures the [Interceptor] to also validate reponses
// in addition to validating requests.
//
// By default:
//
// - Unary: Response messages from the server are not validated.
// - Client streams: Received messages are not validated.
// - Server streams: Sent messages are not validated.
//
// However, these messages are all validated if this option is set.
func WithValidateResponses() Option {
	return optionFunc(func(i *Interceptor) {
		i.validateResponses = true
	})
}

// WithoutErrorDetails configures the [Interceptor] to elide error details from
// validation errors. By default, a [protovalidate.ValidationError] is added
// as a detail when validation errors are returned.
func WithoutErrorDetails() Option {
	return optionFunc(func(i *Interceptor) {
		i.noErrorDetails = true
	})
}

// Interceptor is a [connect.Interceptor] that ensures that RPC request
// messages match the constraints expressed in their Protobuf schemas. It does
// not validate response messages unless the [WithValidateResponses] option
// is specified.
//
// By default, Interceptors use a validator that lazily compiles constraints
// and works with any Protobuf message. This is a simple, widely-applicable
// configuration: after compiling and caching the constraints for a Protobuf
// message type once, validation is very efficient. To customize the validator,
// use [WithValidator] and [protovalidate.ValidatorOption].
//
// RPCs with invalid request messages short-circuit with an error. The error
// always uses [connect.CodeInvalidArgument] and has a [detailed representation
// of the error] attached as a [connect.ErrorDetail].
//
// This interceptor is primarily intended for use on handlers. Client-side use
// is possible, but discouraged unless the client always has an up-to-date
// schema.
//
// [detailed representation of the error]: https://pkg.go.dev/buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate#Violations
type Interceptor struct {
	validator         protovalidate.Validator
	validateResponses bool
	noErrorDetails    bool
}

// NewInterceptor builds an Interceptor. The default configuration is
// appropriate for most use cases.
func NewInterceptor(opts ...Option) *Interceptor {
	var interceptor Interceptor
	for _, opt := range opts {
		opt.apply(&interceptor)
	}

	if interceptor.validator == nil {
		interceptor.validator = protovalidate.GlobalValidator
	}

	return &interceptor
}

// WrapUnary implements connect.Interceptor.
func (i *Interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if err := i.validateRequest(req.Any()); err != nil {
			return nil, err
		}
		response, err := next(ctx, req)
		if err != nil {
			return response, err
		}
		if err := i.validateResponse(response.Any()); err != nil {
			return response, err
		}
		return response, nil
	}
}

// WrapStreamingClient implements connect.Interceptor.
func (i *Interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		return &streamingClientInterceptor{
			StreamingClientConn: next(ctx, spec),
			interceptor:         i,
		}
	}
}

// WrapStreamingHandler implements connect.Interceptor.
func (i *Interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		return next(ctx, &streamingHandlerInterceptor{
			StreamingHandlerConn: conn,
			interceptor:          i,
		})
	}
}

func (i *Interceptor) validateRequest(msg any) error {
	return i.validate(msg, connect.CodeInvalidArgument)
}

func (i *Interceptor) validateResponse(msg any) error {
	if !i.validateResponses {
		return nil
	}
	return i.validate(msg, connect.CodeInternal)
}

func (i *Interceptor) validate(msg any, code connect.Code) error {
	if msg == nil {
		return nil
	}
	protoMsg, ok := msg.(proto.Message)
	if !ok {
		return fmt.Errorf("expected proto.Message, got %T", msg)
	}
	err := i.validator.Validate(protoMsg)
	if err == nil {
		return nil
	}
	connectErr := connect.NewError(code, err)
	if !i.noErrorDetails {
		if validationErr := new(protovalidate.ValidationError); errors.As(err, &validationErr) {
			if detail, err := connect.NewErrorDetail(validationErr.ToProto()); err == nil {
				connectErr.AddDetail(detail)
			}
		}
	}
	return connectErr
}

type streamingClientInterceptor struct {
	connect.StreamingClientConn

	interceptor *Interceptor
}

func (s *streamingClientInterceptor) Send(msg any) error {
	if err := s.interceptor.validateRequest(msg); err != nil {
		return err
	}
	return s.StreamingClientConn.Send(msg)
}

func (s *streamingClientInterceptor) Receive(msg any) error {
	if err := s.StreamingClientConn.Receive(msg); err != nil {
		return err
	}
	return s.interceptor.validateResponse(msg)
}

type streamingHandlerInterceptor struct {
	connect.StreamingHandlerConn

	interceptor *Interceptor
}

func (s *streamingHandlerInterceptor) Send(msg any) error {
	if err := s.interceptor.validateResponse(msg); err != nil {
		return err
	}
	return s.StreamingHandlerConn.Send(msg)
}

func (s *streamingHandlerInterceptor) Receive(msg any) error {
	if err := s.StreamingHandlerConn.Receive(msg); err != nil {
		return err
	}
	return s.interceptor.validateRequest(msg)
}

type optionFunc func(*Interceptor)

func (f optionFunc) apply(i *Interceptor) { f(i) }
