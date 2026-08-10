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

package validate_test

import (
	"context"
	"errors"
	"io"
	"testing"

	validatepb "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"buf.build/go/protovalidate"
	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connectinprocess"
	"connectrpc.com/connect/v2/connectproto"
	"connectrpc.com/validate/v2"
	calculatorv1 "connectrpc.com/validate/v2/internal/gen/example/calculator/v1"
	"connectrpc.com/validate/v2/internal/gen/example/calculator/v1/calculatorv1connect"
	userv1 "connectrpc.com/validate/v2/internal/gen/example/user/v1"
	"connectrpc.com/validate/v2/internal/gen/example/user/v1/userv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestInterceptorUnary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		svc               func(context.Context, *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error)
		req               *userv1.CreateUserRequest
		validateResponses bool
		wantCode          connect.Code
		wantPath          string // field path, from error details
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
			svc:  createUserError,
			req: &userv1.CreateUserRequest{
				User: &userv1.User{Email: "someone@example.com"},
			},
			wantCode: connect.CodeInternal,
		},
		{
			name: "invalid_response",
			svc:  createUserInvalidResponse,
			req: &userv1.CreateUserRequest{
				User: &userv1.User{Email: "foo@foo.com"},
			},
			validateResponses: true,
			wantCode:          connect.CodeInternal,
			wantPath:          "user.email",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var opts []validate.Option
			if test.validateResponses {
				opts = append(opts, validate.WithValidateResponses())
			}
			validator := validate.NewServerInterceptor(opts...)

			server := connect.NewServer(validator)
			userv1connect.RegisterUserServiceHandler(server, &userService{createUser: test.svc})
			client := userv1connect.NewUserServiceClient(
				connect.NewClient(connectinprocess.New(server)),
			)

			got, err := client.CreateUser(t.Context(), test.req)

			if test.wantCode > 0 {
				require.Error(t, err)
				var connectErr *connect.Error
				require.ErrorAs(t, err, &connectErr)
				assert.Equal(t, test.wantCode, connectErr.Code())
				if test.wantPath != "" {
					details := connectErr.Details()
					require.Len(t, details, 1)
					detail, err := connectproto.UnmarshalErrorDetail(details[0])
					require.NoError(t, err)
					violations, ok := detail.(*validatepb.Violations)
					require.True(t, ok)
					require.Len(t, violations.Violations, 1)
					require.Equal(t, test.wantPath, protovalidate.FieldPathString(violations.Violations[0].GetField()))
				}
			} else {
				require.NoError(t, err)
				assert.NotZero(t, got.User)
			}
		})
	}
}

func TestInterceptorStreamingHandler(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		svc               func(context.Context, calculatorv1connect.CalculatorServiceCumSumServerStream) error
		req               *calculatorv1.CumSumRequest
		validateResponses bool
		wantCode          connect.Code
		wantPath          string // field path, from error details
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
		{
			name:              "invalid_response",
			svc:               cumSumInvalidResponse,
			req:               &calculatorv1.CumSumRequest{Number: 1},
			validateResponses: true,
			wantCode:          connect.CodeInternal,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var opts []validate.Option
			if test.validateResponses {
				opts = append(opts, validate.WithValidateResponses())
			}
			validator := validate.NewServerInterceptor(opts...)

			server := connect.NewServer(validator)
			calculatorv1connect.RegisterCalculatorServiceHandler(server, &calculatorService{cumSum: test.svc})
			client := calculatorv1connect.NewCalculatorServiceClient(
				connect.NewClient(connectinprocess.New(server)),
			)

			stream, err := client.CumSum(t.Context())
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = stream.Close()
			})

			// A Send error of io.EOF means the handler failed before reading;
			// Receive surfaces the handler's error.
			err = stream.Send(test.req)
			var got *calculatorv1.CumSumResponse
			if err == nil || errors.Is(err, io.EOF) {
				got, err = stream.Receive()
			}

			if test.wantCode > 0 {
				require.Error(t, err)
				var connectErr *connect.Error
				require.ErrorAs(t, err, &connectErr)
				assert.Equal(t, test.wantCode, connectErr.Code())
				if test.wantPath != "" {
					details := connectErr.Details()
					require.Len(t, details, 1)
					detail, err := connectproto.UnmarshalErrorDetail(details[0])
					require.NoError(t, err)
					violations, ok := detail.(*validatepb.Violations)
					require.True(t, ok)
					require.Len(t, violations.Violations, 1)
					require.Equal(t, test.wantPath, protovalidate.FieldPathString(violations.Violations[0].GetField()))
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
		name              string
		svc               func(context.Context, calculatorv1connect.CalculatorServiceCumSumServerStream) error
		req               *calculatorv1.CumSumRequest
		validateResponses bool
		wantCode          connect.Code
		wantPath          string       // field path, from error details
		wantReceiveCode   connect.Code // code for error calling Receive()
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
		{
			name:              "invalid_response",
			svc:               cumSumInvalidResponse,
			req:               &calculatorv1.CumSumRequest{Number: 1},
			validateResponses: true,
			wantReceiveCode:   connect.CodeInternal,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var opts []validate.Option
			if test.validateResponses {
				opts = append(opts, validate.WithValidateResponses())
			}
			validator := validate.NewClientInterceptor(opts...)

			server := connect.NewServer()
			calculatorv1connect.RegisterCalculatorServiceHandler(server, &calculatorService{cumSum: test.svc})
			client := calculatorv1connect.NewCalculatorServiceClient(
				connect.NewClient(connectinprocess.New(server), validator),
			)

			stream, err := client.CumSum(t.Context())
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = stream.Close()
			})

			err = stream.Send(test.req)
			if errors.Is(err, io.EOF) {
				// The handler failed before reading; Receive surfaces its
				// error.
				err = nil
			}
			if test.wantCode > 0 {
				require.Error(t, err)
				var connectErr *connect.Error
				require.ErrorAs(t, err, &connectErr)
				t.Log(connectErr)
				assert.Equal(t, test.wantCode, connectErr.Code())
				if test.wantPath != "" {
					details := connectErr.Details()
					require.Len(t, details, 1)
					detail, err := connectproto.UnmarshalErrorDetail(details[0])
					require.NoError(t, err)
					violations, ok := detail.(*validatepb.Violations)
					require.True(t, ok)
					require.Len(t, violations.Violations, 1)
					require.Equal(t, test.wantPath, protovalidate.FieldPathString(violations.Violations[0].GetField()))
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
	validator, err := protovalidate.New(protovalidate.WithDisableLazy())
	require.NoError(t, err)
	interceptor := validate.NewServerInterceptor(validate.WithValidator(validator))

	server := connect.NewServer(interceptor)
	userv1connect.RegisterUserServiceHandler(server, &userService{createUser: createUser})
	client := userv1connect.NewUserServiceClient(
		connect.NewClient(connectinprocess.New(server)),
	)

	_, err = client.CreateUser(t.Context(), &userv1.CreateUserRequest{
		User: &userv1.User{Email: "someone@example.com"},
	})
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// userService implements userv1connect.UserServiceHandler with a swappable
// CreateUser implementation.
type userService struct {
	createUser func(context.Context, *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error)
}

func (s *userService) CreateUser(ctx context.Context, req *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {
	return s.createUser(ctx, req)
}

// calculatorService implements calculatorv1connect.CalculatorServiceHandler
// with a swappable CumSum implementation.
type calculatorService struct {
	cumSum func(context.Context, calculatorv1connect.CalculatorServiceCumSumServerStream) error
}

func (s *calculatorService) CumSum(ctx context.Context, stream calculatorv1connect.CalculatorServiceCumSumServerStream) error {
	return s.cumSum(ctx, stream)
}

func createUser(_ context.Context, req *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {
	return &userv1.CreateUserResponse{User: req.User}, nil
}
func createUserError(_ context.Context, _ *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {
	return nil, connect.NewError(connect.CodeInternal, "oh no")
}

func createUserInvalidResponse(_ context.Context, req *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {
	newUser := proto.CloneOf(req.User)
	newUser.Email = "nonsense"
	return &userv1.CreateUserResponse{User: newUser}, nil
}

func cumSumSuccess(_ context.Context, stream calculatorv1connect.CalculatorServiceCumSumServerStream) error {
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

func cumSumError(_ context.Context, _ calculatorv1connect.CalculatorServiceCumSumServerStream) error {
	return connect.NewError(connect.CodeInternal, "boom")
}

func cumSumInvalidResponse(_ context.Context, stream calculatorv1connect.CalculatorServiceCumSumServerStream) error {
	var sum int64
	for {
		req, err := stream.Receive()
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}
		sum += req.Number
		if err := stream.Send(&calculatorv1.CumSumResponse{Sum: sum * -1}); err != nil {
			return err
		}
	}
}
