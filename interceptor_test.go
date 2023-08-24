package validate

import (
	"context"
	"testing"

	"buf.build/gen/go/bufbuild/protovalidate-testing/protocolbuffers/go/buf/validate/conformance/cases"
	"connectrpc.com/connect"

	"github.com/bufbuild/protovalidate-go"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestInterceptor_WrapUnary(t *testing.T) {
	validator, err := protovalidate.New()
	require.NoError(t, err)
	interceptor := NewInterceptor(validator)
	message := &cases.StringConst{Val: "foo"}
	mockUnary := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		return connect.NewResponse(message), nil
	}
	wrappedUnary := interceptor.WrapUnary(mockUnary)
	_, err = wrappedUnary(context.Background(), connect.NewRequest(message))
	assert.NoError(t, err)
}

type mockStreamingClientConn struct {
}

func TestStreamingClientInterceptor_Send(t *testing.T) {
	validator, err := protovalidate.New()
	require.NoError(t, err)
	clientConn := &streamingClientInterceptor{
		validator: validator,
	}
	message := &cases.StringConst{Val: "foo"}
	err = clientConn.Send(connect.NewRequest(message))
	assert.NoError(t, err)
}

func TestStreamingHandlerInterceptor_Receive(t *testing.T) {
	validator, err := protovalidate.New()
	require.NoError(t, err)
	handlerConn := &streamingHandlerInterceptor{
		validator: validator,
	}
	message := &cases.StringConst{Val: "foo"}
	err = handlerConn.Receive(connect.NewRequest(message))
	assert.NoError(t, err)
}
