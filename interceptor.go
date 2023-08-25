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

// Package validate provides an interceptor implementation for the ConnectRPC framework.
// The interceptor integrates with the protovalidate-go library to validate incoming protobuf messages,
// ensuring adherence to the defined message structure.
package validate

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/bufbuild/protovalidate-go"
	"google.golang.org/protobuf/proto"
)

var _ connect.Interceptor = &Interceptor{}

// Option interface is currently empty and serves as a placeholder for potential future implementations.
// It allows adding new options without breaking existing code.
type Option interface {
	unimplemented()
}

// Interceptor implements the connect.Interceptor interface, providing message validation
// for the ConnectRPC framework. It integrates with the protovalidate-go library to ensure
// incoming protocol buffer messages adhere to the defined message structure.
type Interceptor struct {
	validator *protovalidate.Validator
}

// NewInterceptor returns a new instance of the Interceptor.
// It accepts a protovalidate.Validator as a parameter to perform message validation.
func NewInterceptor(
	validator *protovalidate.Validator,
	_ ...Option,
) *Interceptor {
	return &Interceptor{
		validator: validator,
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
	if err := validate(s.validator, msg); err != nil {
		return err
	}
	return s.StreamingHandlerConn.Receive(msg)
}

func validate(validator *protovalidate.Validator, msg any) error {
	switch protoMessage := msg.(type) {
	case connect.AnyRequest:
		return validate(validator, protoMessage.Any())
	case proto.Message:
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
	default:
		return fmt.Errorf("unsupported message type %T", protoMessage)
	}
	return nil
}
