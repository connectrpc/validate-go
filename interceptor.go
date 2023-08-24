package validate

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/bufbuild/protovalidate-go"
	"google.golang.org/protobuf/proto"
)

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

// streamingClientInterceptor implements connect.StreamingClientConn.
type streamingClientInterceptor struct {
	connect.StreamingClientConn
	validator *protovalidate.Validator
}

func (s *streamingClientInterceptor) Send(msg any) error {
	return validate(s.validator, msg)
}

// streamingHandlerInterceptor implements connect.StreamingHandlerConn.
type streamingHandlerInterceptor struct {
	connect.StreamingHandlerConn
	validator *protovalidate.Validator
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
		return err
	}
	return nil
}
