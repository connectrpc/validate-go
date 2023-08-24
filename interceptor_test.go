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
	"testing"

	"buf.build/gen/go/bufbuild/protovalidate-testing/protocolbuffers/go/buf/validate/conformance/cases"
	"connectrpc.com/connect"
	"github.com/bufbuild/protovalidate-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInterceptor_WrapUnary(t *testing.T) {
	t.Parallel()
	var tests = []struct {
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
			validator, err := protovalidate.New()
			require.NoError(t, err)
			interceptor := NewInterceptor(validator)
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

var _ connect.StreamingClientConn = &mockStreamingClientConn{}

type mockStreamingClientConn struct {
	connect.StreamingClientConn
}

func (m *mockStreamingClientConn) Send(_ any) error {
	return nil
}

func TestStreamingClientInterceptor_Send(t *testing.T) {
	t.Parallel()
	var tests = []struct {
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
			validator, err := protovalidate.New()
			require.NoError(t, err)

			clientConn := streamingClientInterceptor{
				validator:           validator,
				StreamingClientConn: &mockStreamingClientConn{},
			}
			err = clientConn.Send(connect.NewRequest(test.message))
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

var _ connect.StreamingHandlerConn = &mockStreamingHandlerConn{}

type mockStreamingHandlerConn struct {
	connect.StreamingHandlerConn
}

func (m *mockStreamingHandlerConn) Receive(_ any) error {
	return nil
}

func TestStreamingHandlerInterceptor_Receive(t *testing.T) {
	t.Parallel()
	var tests = []struct {
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
			validator, err := protovalidate.New()
			require.NoError(t, err)
			handlerConn := &streamingHandlerInterceptor{
				validator:            validator,
				StreamingHandlerConn: &mockStreamingHandlerConn{},
			}
			err = handlerConn.Receive(connect.NewRequest(test.message))
			if test.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
