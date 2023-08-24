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

package validate

import (
	"context"
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

// Interceptor implements connect.Interceptor.
type Interceptor struct {
	validator *protovalidate.Validator
}

// NewInterceptor returns a new Interceptor.
func NewInterceptor(
	validator *protovalidate.Validator,
	_ ...Option,
) *Interceptor {
	return &Interceptor{
		validator: validator,
	}
}

// WrapUnary returns a new connect.UnaryFunc that wraps the given connect.UnaryFunc.
func (i *Interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if err := validate(i.validator, req.Any()); err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

// WrapStreamingClient returns a new connect.StreamingClientFunc that wraps the given connect.StreamingClientFunc.
func (i *Interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		return &streamingClientInterceptor{
			validator:           i.validator,
			StreamingClientConn: next(ctx, spec),
		}
	}
}

// WrapStreamingHandler returns a new connect.StreamingHandlerFunc that wraps the given connect.StreamingHandlerFunc.
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
	switch m := msg.(type) {
	case connect.AnyRequest:
		return validate(validator, m.Any())
	case proto.Message:
		if err := validator.Validate(m); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported message type")
	}
	return nil
}
