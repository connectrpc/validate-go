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

package validate_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	validateproto "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"connectrpc.com/connect"
	"connectrpc.com/validate"
	pingv1 "connectrpc.com/validate/internal/gen/connect/ping/v1"
	"connectrpc.com/validate/internal/gen/connect/ping/v1/pingv1connect"
	"connectrpc.com/validate/internal/testserver"
	"github.com/bufbuild/protovalidate-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInterceptor(t *testing.T) {
	t.Parallel()
	t.Run("success", func(t *testing.T) {
		t.Parallel()
		interceptor, err := validate.NewInterceptor()
		require.NoError(t, err)
		assert.NotNil(t, interceptor)
	})
	t.Run("success with validator", func(t *testing.T) {
		t.Parallel()
		validator, err := protovalidate.New()
		require.NoError(t, err)
		interceptor, err := validate.NewInterceptor(validate.WithValidator(validator))
		require.NoError(t, err)
		assert.NotNil(t, interceptor)
	})
}

func TestInterceptor_WrapUnary(t *testing.T) {
	t.Parallel()
	type args struct {
		msg    string
		code   connect.Code
		detail *protovalidate.ValidationError
	}
	tests := []struct {
		name    string
		svc     pingv1connect.PingServiceHandler
		req     *pingv1.PingRequest
		want    *pingv1.PingResponse
		wantErr *args
	}{
		{
			name: "empty request returns error on required request fields",
			req:  &pingv1.PingRequest{},
			wantErr: &args{
				msg:  "validation error:\n - number: value is required [required]",
				code: connect.CodeInvalidArgument,
				detail: &protovalidate.ValidationError{
					Violations: []*validateproto.Violation{
						{
							FieldPath:    "number",
							ConstraintId: "required",
							Message:      "value is required",
						},
					},
				},
			},
		},
		{
			name: "invalid request returns error with constraint violation",
			req: &pingv1.PingRequest{
				Number: 123,
			},
			wantErr: &args{
				msg:  "validation error:\n - number: value must be greater than 0 and less than 100 [int64.gt_lt]",
				code: connect.CodeInvalidArgument,
				detail: &protovalidate.ValidationError{
					Violations: []*validateproto.Violation{
						{
							FieldPath:    "number",
							ConstraintId: "int64.gt_lt",
							Message:      "value must be greater than 0 and less than 100",
						},
					},
				},
			},
		},
		{
			name: "unrelated server error remains unaffected",
			svc: testserver.NewPingServer(
				testserver.WithErr(
					connect.NewError(connect.CodeInternal, fmt.Errorf("oh no")),
				),
			),
			req: &pingv1.PingRequest{
				Number: 50,
			},
			wantErr: &args{
				msg:  "oh no",
				code: connect.CodeInternal,
			},
		},
		{
			name: "valid request returns response",
			req: &pingv1.PingRequest{
				Number: 50,
			},
			want: &pingv1.PingResponse{
				Number: 50,
			},
		},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if test.svc == nil {
				test.svc = testserver.NewPingServer()
			}

			validator, err := validate.NewInterceptor()
			require.NoError(t, err)

			mux := http.NewServeMux()
			mux.Handle(pingv1connect.NewPingServiceHandler(
				test.svc,
				connect.WithInterceptors(validator),
			))

			exampleBookingServer := testserver.NewInMemoryServer(mux)
			defer exampleBookingServer.Close()

			client := pingv1connect.NewPingServiceClient(
				exampleBookingServer.Client(),
				exampleBookingServer.URL(),
			)

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			got, err := client.Ping(ctx, connect.NewRequest(test.req))
			if test.wantErr != nil {
				require.Error(t, err)
				var connectErr *connect.Error
				assert.True(t, errors.As(err, &connectErr))
				assert.Equal(t, test.wantErr.msg, connectErr.Message())
				assert.Equal(t, test.wantErr.code, connectErr.Code())
				if test.wantErr.detail != nil {
					require.Len(t, connectErr.Details(), 1)
					detail, err := connect.NewErrorDetail(test.wantErr.detail.ToProto())
					require.NoError(t, err)
					assert.Equal(t, connectErr.Details()[0].Type(), detail.Type())
				}
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.want.GetNumber(), got.Msg.GetNumber())
			}
		})
	}
}

func TestInterceptor_WrapStreamingClient(t *testing.T) {
	t.Parallel()
	type args struct {
		msg      string
		code     connect.Code
		detail   *protovalidate.ValidationError
		closeErr bool
	}
	tests := []struct {
		name    string
		svc     pingv1connect.PingServiceHandler
		req     *pingv1.SumRequest
		want    *pingv1.SumResponse
		wantErr *args
	}{
		{
			name: "empty request returns error on required request fields",
			req:  &pingv1.SumRequest{},
			wantErr: &args{
				msg:  "validation error:\n - number: value is required [required]",
				code: connect.CodeInvalidArgument,
				detail: &protovalidate.ValidationError{
					Violations: []*validateproto.Violation{
						{
							FieldPath:    "number",
							ConstraintId: "required",
							Message:      "value is required",
						},
					},
				},
			},
		},
		{
			name: "invalid request returns error with constraint violation",
			req: &pingv1.SumRequest{
				Number: 123,
			},
			wantErr: &args{
				msg:  "validation error:\n - number: value must be greater than 0 and less than 100 [int64.gt_lt]",
				code: connect.CodeInvalidArgument,
				detail: &protovalidate.ValidationError{
					Violations: []*validateproto.Violation{
						{
							FieldPath:    "number",
							ConstraintId: "int64.gt_lt",
							Message:      "value must be greater than 0 and less than 100",
						},
					},
				},
			},
		},
		{
			name: "unrelated server error remains unaffected",
			svc: testserver.NewPingServer(
				testserver.WithErr(
					connect.NewError(connect.CodeInternal, fmt.Errorf("oh no")),
				),
			),
			req: &pingv1.SumRequest{
				Number: 50,
			},
			wantErr: &args{
				msg:      "oh no",
				code:     connect.CodeInternal,
				closeErr: true,
			},
		},
		{
			name: "valid request returns response",
			req: &pingv1.SumRequest{
				Number: 50,
			},
			want: &pingv1.SumResponse{
				Sum: 50,
			},
		},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if test.svc == nil {
				test.svc = testserver.NewPingServer()
			}

			validator, err := validate.NewInterceptor()
			require.NoError(t, err)

			mux := http.NewServeMux()
			mux.Handle(pingv1connect.NewPingServiceHandler(
				test.svc,
			))

			exampleBookingServer := testserver.NewInMemoryServer(mux)
			defer exampleBookingServer.Close()

			client := pingv1connect.NewPingServiceClient(
				exampleBookingServer.Client(),
				exampleBookingServer.URL(),
				connect.WithInterceptors(validator),
			)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			stream := client.Sum(ctx)
			err = stream.Send(test.req)
			resp, closeErr := stream.CloseAndReceive()
			if test.wantErr != nil {
				if test.wantErr.closeErr {
					err = closeErr
				}
				require.Error(t, err)
				var connectErr *connect.Error
				assert.True(t, errors.As(err, &connectErr))
				assert.Equal(t, test.wantErr.msg, connectErr.Message())
				assert.Equal(t, test.wantErr.code, connectErr.Code())
				if test.wantErr.detail != nil {
					require.Len(t, connectErr.Details(), 1)
					detail, err := connect.NewErrorDetail(test.wantErr.detail.ToProto())
					require.NoError(t, err)
					assert.Equal(t, connectErr.Details()[0].Type(), detail.Type())
				}
			} else {
				require.NoError(t, err)
				require.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, resp.Msg.GetSum(), test.want.GetSum())
			}
		})
	}
}

func TestInterceptor_WrapStreamingHandler(t *testing.T) {
	t.Parallel()
	type args struct {
		msg      string
		code     connect.Code
		detail   *protovalidate.ValidationError
		closeErr bool
	}
	tests := []struct {
		name    string
		svc     pingv1connect.PingServiceHandler
		req     *pingv1.CountUpRequest
		want    *pingv1.CountUpResponse
		wantErr *args
	}{
		{
			name: "empty request returns error on required request fields",
			req:  &pingv1.CountUpRequest{},
			wantErr: &args{
				msg:  "validation error:\n - number: value is required [required]",
				code: connect.CodeInvalidArgument,
				detail: &protovalidate.ValidationError{
					Violations: []*validateproto.Violation{
						{
							FieldPath:    "number",
							ConstraintId: "required",
							Message:      "value is required",
						},
					},
				},
			},
		},
		{
			name: "invalid request returns error with constraint violation",
			req: &pingv1.CountUpRequest{
				Number: 123,
			},
			wantErr: &args{
				msg:  "validation error:\n - number: value must be greater than 0 and less than 100 [int64.gt_lt]",
				code: connect.CodeInvalidArgument,
				detail: &protovalidate.ValidationError{
					Violations: []*validateproto.Violation{
						{
							FieldPath:    "number",
							ConstraintId: "int64.gt_lt",
							Message:      "value must be greater than 0 and less than 100",
						},
					},
				},
			},
		},
		{
			name: "unrelated server error remains unaffected",
			svc: testserver.NewPingServer(
				testserver.WithErr(
					connect.NewError(connect.CodeInternal, fmt.Errorf("oh no")),
				),
			),
			req: &pingv1.CountUpRequest{
				Number: 50,
			},
			wantErr: &args{
				msg:      "oh no",
				code:     connect.CodeInternal,
				closeErr: true,
			},
		},
		{
			name: "valid request returns response",
			req: &pingv1.CountUpRequest{
				Number: 50,
			},
			want: &pingv1.CountUpResponse{
				Number: 1,
			},
		},
	}
	for _, tt := range tests {
		test := tt
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if test.svc == nil {
				test.svc = testserver.NewPingServer()
			}
			validator, err := validate.NewInterceptor()
			require.NoError(t, err)
			mux := http.NewServeMux()
			mux.Handle(pingv1connect.NewPingServiceHandler(
				test.svc,
				connect.WithInterceptors(validator),
			))

			exampleBookingServer := testserver.NewInMemoryServer(mux)
			defer exampleBookingServer.Close()

			client := pingv1connect.NewPingServiceClient(
				exampleBookingServer.Client(),
				exampleBookingServer.URL(),
			)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			stream, err := client.CountUp(ctx, connect.NewRequest(test.req))
			require.NoError(t, err)
			receive := stream.Receive()
			got := stream.Msg()
			err = stream.Err()
			if test.wantErr != nil {
				require.Error(t, err)
				var connectErr *connect.Error
				assert.True(t, errors.As(err, &connectErr))
				assert.Equal(t, test.wantErr.msg, connectErr.Message())
				assert.Equal(t, test.wantErr.code, connectErr.Code())
				if test.wantErr.detail != nil {
					require.Len(t, connectErr.Details(), 1)
					detail, err := connect.NewErrorDetail(test.wantErr.detail.ToProto())
					require.NoError(t, err)
					assert.Equal(t, connectErr.Details()[0].Type(), detail.Type())
				}
			} else {
				require.NoError(t, err)
				assert.True(t, receive)
				assert.NotNil(t, got)
				assert.Equal(t, test.want.GetNumber(), got.GetNumber())
			}
		})
	}
}
