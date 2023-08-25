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
	"testing"

	"github.com/bufbuild/protovalidate-go"

	"buf.build/gen/go/bufbuild/protovalidate-testing/protocolbuffers/go/buf/validate/conformance/cases"
	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInterceptor(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		interceptor, err := NewInterceptor()
		require.NoError(t, err)
		assert.NotNil(t, interceptor.validator)
	})
	t.Run("success with validator", func(t *testing.T) {
		validator, err := protovalidate.New()
		require.NoError(t, err)
		interceptor, err := NewInterceptor(WithInterceptor(validator))
		require.NoError(t, err)
		assert.NotNil(t, interceptor.validator)
		assert.Equal(t, interceptor.validator, validator)
	})
}

func TestInterceptor_WrapUnary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		message *cases.StringConst
		wantErr bool
	}{
		{
			name:    "success",
			message: &cases.StringConst{Val: "foo"},
			wantErr: false,
		},
		{
			name:    "fail",
			message: &cases.StringConst{Val: "bar"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			interceptor, err := NewInterceptor()
			require.NoError(t, err)
			mockUnary := func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				return nil, nil
			}
			_, err = interceptor.WrapUnary(mockUnary)(
				context.Background(),
				connect.NewRequest(test.message),
			)
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

var _ connect.StreamingClientConn = (*mockStreamingClientConn)(nil)

type mockStreamingClientConn struct {
	connect.StreamingClientConn

	sendFunc func(any) error
}

func (m *mockStreamingClientConn) Send(in any) error {
	return m.sendFunc(in)
}

var streamingTests = []struct {
	name    string
	message any
	mock    func(any) error
	wantErr string
}{
	{
		name:    "success",
		message: &cases.StringConst{Val: "foo"},
		mock: func(a any) error {
			return nil
		},
	},
	{
		name:    "fail validation",
		message: &cases.StringConst{Val: "bar"},
		mock: func(a any) error {
			return nil
		},
		wantErr: "invalid_argument: validation error:\n - val: value must equal `foo` [string.const]",
	},
	{
		name:    "fail not a proto.Message",
		message: struct{ name string }{name: "baz"},
		mock: func(any) error {
			return nil
		},
		wantErr: "message is not a proto.Message: struct { name string }",
	},
	{
		name:    "pass validation and fail send",
		message: &cases.StringConst{Val: "foo"},
		mock: func(any) error {
			return fmt.Errorf("send error")
		},
		wantErr: "send error",
	},
}

func TestStreamingClientInterceptor_Send(t *testing.T) {
	t.Parallel()
	for _, tt := range streamingTests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			interceptor, err := NewInterceptor()
			require.NoError(t, err)

			next := func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
				return &mockStreamingClientConn{
					sendFunc: test.mock,
				}
			}

			client := interceptor.WrapStreamingClient(next)
			conn := client(context.Background(), connect.Spec{})
			err = conn.Send(test.message)
			if test.wantErr != "" {
				require.Error(t, err)
				assert.EqualError(t, err, test.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

var _ connect.StreamingHandlerConn = (*mockStreamingHandlerConn)(nil)

type mockStreamingHandlerConn struct {
	connect.StreamingHandlerConn

	receiveFunc func(any) error
}

func (m *mockStreamingHandlerConn) Receive(in any) error {
	return m.receiveFunc(in)
}

func TestStreamingHandlerInterceptor_Receive(t *testing.T) {
	t.Parallel()
	for _, tt := range streamingTests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			i, err := NewInterceptor()
			require.NoError(t, err)
			next := func(ctx context.Context, conn connect.StreamingHandlerConn) error {
				return conn.Receive(test.message)
			}
			conn := &mockStreamingHandlerConn{
				receiveFunc: test.mock,
			}
			err = i.WrapStreamingHandler(next)(context.Background(), conn)
			if test.wantErr != "" {
				require.Error(t, err)
				assert.EqualError(t, err, test.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
