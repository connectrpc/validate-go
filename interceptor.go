package validate_go

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/bufbuild/protovalidate-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type Interceptor struct {
	validator *protovalidate.Validator
}

func NewInterceptor(validator *protovalidate.Validator, _ ...Option) *Interceptor {
	return &Interceptor{validator: validator}
}

func (i *Interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return NewUnaryFunc(i.validator)(next)
}

func (i *Interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return NewStreamingClientFunc(i.validator)(next)
}

func (i *Interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return NewStreamingHandlerFunc(i.validator)(next)
}

// Option interface is currently empty and serves as a placeholder for potential future implementations.
// It allows adding new options without breaking existing code.
type Option interface {
	unimplemented()
}

// NewUnaryFunc returns a new UnaryFunc that validates the request and response
func NewUnaryFunc(validator *protovalidate.Validator, _ ...Option) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if err := validate(validator, req.Any()); err != nil {
				return nil, err
			}
			return next(ctx, req)
		}
	}
}

// NewStreamingClientFunc returns a new StreamingClientFunc that validates the request and response
func NewStreamingClientFunc(validator *protovalidate.Validator, _ ...Option) func(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(next connect.StreamingClientFunc) connect.StreamingClientFunc {
		return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
			return &streamingClientInterceptor{
				validator:           validator,
				StreamingClientConn: next(ctx, spec),
			}
		}
	}
}

type streamingClientInterceptor struct {
	validator *protovalidate.Validator
	connect.StreamingClientConn
}

func (s *streamingClientInterceptor) Receive(msg any) error {
	return validate(s.validator, msg)
}

// NewStreamingHandlerFunc returns a new StreamingHandlerFunc that validates the request and response
func NewStreamingHandlerFunc(validator *protovalidate.Validator, _ ...Option) func(connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
		return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
			return next(ctx, &streamingHandlerInterceptor{
				validator:            validator,
				StreamingHandlerConn: conn,
			})
		}
	}
}

type streamingHandlerInterceptor struct {
	validator *protovalidate.Validator
	connect.StreamingHandlerConn
}

func (s *streamingHandlerInterceptor) Receive(msg any) error {
	return validate(s.validator, msg)
}

func validate(validator *protovalidate.Validator, msg any) error {
	message, ok := msg.(proto.Message)
	if !ok {
		return errors.New("unsupported message type")
	}
	if err := validator.Validate(message); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	return nil
}
