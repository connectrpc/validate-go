// Copyright 2023-2024 The Connect Authors
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
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	validatepb "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"connectrpc.com/connect"
	"connectrpc.com/validate"
	calculatorv1 "connectrpc.com/validate/internal/gen/example/calculator/v1"
	"connectrpc.com/validate/internal/gen/example/calculator/v1/calculatorv1connect"
	userv1 "connectrpc.com/validate/internal/gen/example/user/v1"
	"connectrpc.com/validate/internal/gen/example/user/v1/userv1connect"
	"github.com/bufbuild/protovalidate-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInterceptorUnary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		svc      func(context.Context, *connect.Request[userv1.CreateUserRequest]) (*connect.Response[userv1.CreateUserResponse], error)
		req      *userv1.CreateUserRequest
		wantCode connect.Code
		wantPath string // field path, from error details
	}{
		{
			name: "valid",
			svc:  createUser,
			req: &userv1.CreateUserRequest{
				User: &userv1.User{Email: "someone@example.com"},
			},
		},
		{
			name: "invalid",
			req: &userv1.CreateUserRequest{
				User: &userv1.User{Email: "foo"},
			},
			wantCode: connect.CodeInvalidArgument,
			wantPath: "user.email",
		},
		{
			name: "underlying_error",
			svc: func(_ context.Context, _ *connect.Request[userv1.CreateUserRequest]) (*connect.Response[userv1.CreateUserResponse], error) {
				return nil, connect.NewError(connect.CodeInternal, errors.New("oh no"))
			},
			req: &userv1.CreateUserRequest{
				User: &userv1.User{Email: "someone@example.com"},
			},
			wantCode: connect.CodeInternal,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			validator, err := validate.NewInterceptor()
			require.NoError(t, err)

			mux := http.NewServeMux()
			mux.Handle(userv1connect.UserServiceCreateUserProcedure, connect.NewUnaryHandler(
				userv1connect.UserServiceCreateUserProcedure,
				test.svc,
				connect.WithInterceptors(validator),
			))
			srv := startHTTPServer(t, mux)

			got, err := userv1connect.NewUserServiceClient(srv.Client(), srv.URL).
				CreateUser(context.Background(), connect.NewRequest(test.req))

			if test.wantCode > 0 {
				require.Error(t, err)
				var connectErr *connect.Error
				require.ErrorAs(t, err, &connectErr)
				assert.Equal(t, test.wantCode, connectErr.Code())
				if test.wantPath != "" {
					details := connectErr.Details()
					require.Len(t, details, 1)
					detail, err := details[0].Value()
					require.NoError(t, err)
					violations, ok := detail.(*validatepb.Violations)
					require.True(t, ok)
					require.Len(t, violations.Violations, 1)
					require.Equal(t, test.wantPath, violations.Violations[0].FieldPath)
				}
			} else {
				require.NoError(t, err)
				assert.NotZero(t, got.Msg)
			}
		})
	}
}

func TestInterceptorStreamingHandler(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		svc      func(context.Context, *connect.BidiStream[calculatorv1.CumSumRequest, calculatorv1.CumSumResponse]) error
		req      *calculatorv1.CumSumRequest
		wantCode connect.Code
		wantPath string // field path, from error details
	}{
		{
			name:     "invalid",
			svc:      cumSumSuccess,
			req:      &calculatorv1.CumSumRequest{Number: 0},
			wantCode: connect.CodeInvalidArgument,
			wantPath: "number",
		},
		{
			name: "valid",
			svc:  cumSumSuccess,
			req:  &calculatorv1.CumSumRequest{Number: 1},
		},
		{
			name:     "underlying_error",
			svc:      cumSumError,
			req:      &calculatorv1.CumSumRequest{Number: 1},
			wantCode: connect.CodeInternal,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			validator, err := validate.NewInterceptor()
			require.NoError(t, err)

			mux := http.NewServeMux()
			mux.Handle(calculatorv1connect.CalculatorServiceCumSumProcedure, connect.NewBidiStreamHandler(
				calculatorv1connect.CalculatorServiceCumSumProcedure,
				test.svc,
				connect.WithInterceptors(validator),
			))
			srv := httptest.NewUnstartedServer(mux)
			srv.EnableHTTP2 = true
			srv.StartTLS()
			t.Cleanup(srv.Close)

			client := calculatorv1connect.NewCalculatorServiceClient(srv.Client(), srv.URL)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			t.Cleanup(cancel)
			stream := client.CumSum(ctx)
			t.Cleanup(func() {
				assert.NoError(t, stream.CloseResponse())
			})
			t.Cleanup(func() {
				assert.NoError(t, stream.CloseRequest())
			})

			err = stream.Send(test.req)
			require.NoError(t, err)
			time.Sleep(time.Second)
			got, err := stream.Receive()

			if test.wantCode > 0 {
				require.Error(t, err)
				var connectErr *connect.Error
				require.ErrorAs(t, err, &connectErr)
				assert.Equal(t, test.wantCode, connectErr.Code())
				if test.wantPath != "" {
					details := connectErr.Details()
					require.Len(t, details, 1)
					detail, err := details[0].Value()
					require.NoError(t, err)
					violations, ok := detail.(*validatepb.Violations)
					require.True(t, ok)
					require.Len(t, violations.Violations, 1)
					require.Equal(t, test.wantPath, violations.Violations[0].FieldPath)
				}
			} else {
				require.NoError(t, err)
				require.NotZero(t, got.Sum)
			}
		})
	}
}

func TestInterceptorStreamingClient(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		svc             func(context.Context, *connect.BidiStream[calculatorv1.CumSumRequest, calculatorv1.CumSumResponse]) error
		req             *calculatorv1.CumSumRequest
		wantCode        connect.Code
		wantPath        string       // field path, from error details
		wantReceiveCode connect.Code // code for error calling Receive()
	}{
		{
			name:     "invalid",
			svc:      cumSumSuccess,
			req:      &calculatorv1.CumSumRequest{Number: 0},
			wantCode: connect.CodeInvalidArgument,
			wantPath: "number",
		},
		{
			name: "valid",
			svc:  cumSumSuccess,
			req:  &calculatorv1.CumSumRequest{Number: 1},
		},
		{
			name:            "underlying_error",
			svc:             cumSumError,
			req:             &calculatorv1.CumSumRequest{Number: 1},
			wantReceiveCode: connect.CodeInternal,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			validator, err := validate.NewInterceptor()
			require.NoError(t, err)

			mux := http.NewServeMux()
			mux.Handle(calculatorv1connect.CalculatorServiceCumSumProcedure, connect.NewBidiStreamHandler(
				calculatorv1connect.CalculatorServiceCumSumProcedure,
				test.svc,
			))
			srv := httptest.NewUnstartedServer(mux)
			srv.EnableHTTP2 = true
			srv.StartTLS()
			t.Cleanup(srv.Close)

			client := calculatorv1connect.NewCalculatorServiceClient(
				srv.Client(),
				srv.URL,
				connect.WithInterceptors(validator),
			)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			t.Cleanup(cancel)
			stream := client.CumSum(ctx)
			t.Cleanup(func() {
				assert.NoError(t, stream.CloseResponse())
			})
			t.Cleanup(func() {
				assert.NoError(t, stream.CloseRequest())
			})

			err = stream.Send(test.req)
			if test.wantCode > 0 {
				require.Error(t, err)
				var connectErr *connect.Error
				require.ErrorAs(t, err, &connectErr)
				t.Log(connectErr)
				assert.Equal(t, test.wantCode, connectErr.Code())
				if test.wantPath != "" {
					details := connectErr.Details()
					require.Len(t, details, 1)
					detail, err := details[0].Value()
					require.NoError(t, err)
					violations, ok := detail.(*validatepb.Violations)
					require.True(t, ok)
					require.Len(t, violations.Violations, 1)
					require.Equal(t, test.wantPath, violations.Violations[0].FieldPath)
				}
			} else {
				require.NoError(t, err)
				got, receiveErr := stream.Receive()
				if test.wantReceiveCode > 0 {
					require.Equal(t, test.wantReceiveCode, connect.CodeOf(receiveErr))
				} else {
					require.NoError(t, receiveErr)
					require.NotZero(t, got.Sum)
				}
			}
		})
	}
}

func TestWithValidator(t *testing.T) {
	t.Parallel()
	validator, err := protovalidate.New(protovalidate.WithDisableLazy(true))
	require.NoError(t, err)
	interceptor, err := validate.NewInterceptor(validate.WithValidator(validator))
	require.NoError(t, err)

	mux := http.NewServeMux()
	mux.Handle(userv1connect.UserServiceCreateUserProcedure, connect.NewUnaryHandler(
		userv1connect.UserServiceCreateUserProcedure,
		createUser,
		connect.WithInterceptors(interceptor),
	))
	srv := startHTTPServer(t, mux)

	req := connect.NewRequest(&userv1.CreateUserRequest{
		User: &userv1.User{Email: "someone@example.com"},
	})
	_, err = userv1connect.NewUserServiceClient(srv.Client(), srv.URL).
		CreateUser(context.Background(), req)
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func startHTTPServer(tb testing.TB, h http.Handler) *httptest.Server {
	tb.Helper()
	srv := httptest.NewUnstartedServer(h)
	srv.EnableHTTP2 = true
	srv.Start()
	tb.Cleanup(srv.Close)
	return srv
}

func createUser(_ context.Context, req *connect.Request[userv1.CreateUserRequest]) (*connect.Response[userv1.CreateUserResponse], error) {
	return connect.NewResponse(&userv1.CreateUserResponse{User: req.Msg.User}), nil
}

func cumSumSuccess(_ context.Context, stream *connect.BidiStream[calculatorv1.CumSumRequest, calculatorv1.CumSumResponse]) error {
	var sum int64
	for {
		req, err := stream.Receive()
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}
		sum += req.Number
		if err := stream.Send(&calculatorv1.CumSumResponse{Sum: sum}); err != nil {
			return err
		}
	}
}

func cumSumError(_ context.Context, _ *connect.BidiStream[calculatorv1.CumSumRequest, calculatorv1.CumSumResponse]) error {
	return connect.NewError(connect.CodeInternal, errors.New("boom"))
}
