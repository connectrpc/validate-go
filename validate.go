// Copyright 2023-2026 The Connect Authors
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

// Package validate provides connect-go v2 interceptors that validate messages
// against constraints specified in their Protobuf schemas. Because the
// interceptors are powered by [protovalidate], validation is flexible,
// efficient, and consistent across languages - without additional code
// generation.
//
// Server-side use:
//
//	server := connect.NewServer(validate.NewServerInterceptor())
//
// Client-side use (discouraged unless the client always has an up-to-date
// schema):
//
//	client := pingv1connect.NewPingServiceClient(
//	    connect.NewClient(transport, validate.NewClientInterceptor()),
//	)
package validate

import (
	"context"
	"errors"
	"fmt"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connectproto"
	"google.golang.org/protobuf/proto"
)

// An Option configures an interceptor built by [NewServerInterceptor] or
// [NewClientInterceptor].
type Option interface {
	apply(*config)
}

// WithValidator configures the interceptor to use a customized
// [protovalidate.Validator]. By default, [protovalidate.GlobalValidator]
// is used. See [protovalidate.ValidatorOption] for the range of available
// customizations.
func WithValidator(validator protovalidate.Validator) Option {
	return optionFunc(func(c *config) {
		c.validator = validator
	})
}

// WithValidateResponses configures the interceptor to also validate responses
// in addition to validating requests.
//
// By default:
//
//   - Unary: Response messages from the server are not validated.
//   - Client streams: Received messages are not validated.
//   - Server streams: Sent messages are not validated.
//
// However, these messages are all validated if this option is set.
func WithValidateResponses() Option {
	return optionFunc(func(c *config) {
		c.validateResponses = true
	})
}

// WithoutErrorDetails configures the interceptor to elide error details from
// validation errors. By default, a [protovalidate.ValidationError] is added
// as a detail when validation errors are returned.
func WithoutErrorDetails() Option {
	return optionFunc(func(c *config) {
		c.noErrorDetails = true
	})
}

// NewServerInterceptor returns a [connect.ServerInterceptor] that
// validates request messages received from the client. When
// [WithValidateResponses] is set, it also validates response messages sent
// by the server.
//
// Invalid request messages short-circuit with
// [connect.CodeInvalidArgument]. Invalid response messages short-circuit
// with [connect.CodeInternal]. Both errors carry a [detailed
// representation of the error] as an error detail unless
// [WithoutErrorDetails] is set.
//
// [detailed representation of the error]: https://pkg.go.dev/buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate#Violations
func NewServerInterceptor(opts ...Option) connect.ServerInterceptor {
	cfg := newConfig(opts)
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			return next(ctx, spec, &serverStream{ServerStream: stream, config: cfg})
		}
	}
}

// NewClientInterceptor returns a [connect.ClientInterceptor] that
// validates request messages sent to the server. When
// [WithValidateResponses] is set, it also validates response messages
// received from the server.
//
// Client-side use is discouraged unless the client always has an
// up-to-date schema; prefer running validation on the server.
//
// Invalid request messages short-circuit with
// [connect.CodeInvalidArgument]. Invalid response messages short-circuit
// with [connect.CodeInternal]. Both errors carry a [detailed
// representation of the error] as an error detail unless
// [WithoutErrorDetails] is set.
//
// [detailed representation of the error]: https://pkg.go.dev/buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate#Violations
func NewClientInterceptor(opts ...Option) connect.ClientInterceptor {
	cfg := newConfig(opts)
	return func(next connect.ClientFunc) connect.ClientFunc {
		return func(ctx context.Context, spec connect.Spec) (connect.ClientStream, error) {
			stream, err := next(ctx, spec)
			if err != nil {
				return nil, err
			}
			return &clientStream{ClientStream: stream, config: cfg}, nil
		}
	}
}

type config struct {
	validator         protovalidate.Validator
	validateResponses bool
	noErrorDetails    bool
}

func newConfig(opts []Option) *config {
	cfg := &config{}
	for _, opt := range opts {
		opt.apply(cfg)
	}
	if cfg.validator == nil {
		cfg.validator = protovalidate.GlobalValidator
	}
	return cfg
}

func (c *config) validateRequest(msg any) error {
	return c.validate(msg, connect.CodeInvalidArgument)
}

func (c *config) validateResponse(msg any) error {
	if !c.validateResponses {
		return nil
	}
	return c.validate(msg, connect.CodeInternal)
}

func (c *config) validate(msg any, code connect.Code) error {
	if msg == nil {
		return nil
	}
	protoMsg, ok := msg.(proto.Message)
	if !ok {
		return fmt.Errorf("expected proto.Message, got %T", msg)
	}
	err := c.validator.Validate(protoMsg)
	if err == nil {
		return nil
	}
	connectErr := connect.Errorf(code, "%s", err.Error()).WithCause(err)
	if !c.noErrorDetails {
		if validationErr := new(protovalidate.ValidationError); errors.As(err, &validationErr) {
			if detail, err := connectproto.NewErrorDetail(validationErr.ToProto()); err == nil {
				connectErr = connectErr.WithDetail(detail)
			}
		}
	}
	return connectErr
}

// serverStream wraps a [connect.ServerStream] to validate request
// messages on Receive and, when configured, response messages on Send.
type serverStream struct {
	connect.ServerStream

	config *config
}

func (s *serverStream) Receive(msg any) error {
	if err := s.ServerStream.Receive(msg); err != nil {
		return err
	}
	return s.config.validateRequest(msg)
}

func (s *serverStream) Send(msg any) error {
	if err := s.config.validateResponse(msg); err != nil {
		return err
	}
	return s.ServerStream.Send(msg)
}

// clientStream wraps a [connect.ClientStream] to validate request
// messages on Send and, when configured, response messages on Receive.
type clientStream struct {
	connect.ClientStream

	config *config
}

func (s *clientStream) Send(msg any) error {
	if err := s.config.validateRequest(msg); err != nil {
		return err
	}
	return s.ClientStream.Send(msg)
}

func (s *clientStream) Receive(msg any) error {
	if err := s.ClientStream.Receive(msg); err != nil {
		return err
	}
	return s.config.validateResponse(msg)
}

type optionFunc func(*config)

func (f optionFunc) apply(c *config) { f(c) }
