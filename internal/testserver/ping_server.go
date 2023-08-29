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

package testserver

import (
	"context"

	"connectrpc.com/connect"
	pingv1 "connectrpc.com/validate/internal/gen/connect/ping/v1"
	"connectrpc.com/validate/internal/gen/connect/ping/v1/pingv1connect"
)

type pingServer struct {
	pingv1connect.UnimplementedPingServiceHandler

	err error
}

type Option func(*pingServer)

func NewPingServer(opts ...Option) pingv1connect.PingServiceHandler {
	out := &pingServer{}
	for _, apply := range opts {
		apply(out)
	}
	return out
}

func WithErr(err error) Option {
	return func(p *pingServer) {
		p.err = err
	}
}

func (p *pingServer) Ping(_ context.Context, req *connect.Request[pingv1.PingRequest]) (*connect.Response[pingv1.PingResponse], error) {
	if p.err != nil {
		return nil, p.err
	}
	return connect.NewResponse(&pingv1.PingResponse{
		Number: req.Msg.GetNumber(),
	}), nil
}

func (p *pingServer) Sum(_ context.Context, stream *connect.ClientStream[pingv1.SumRequest]) (*connect.Response[pingv1.SumResponse], error) {
	if p.err != nil {
		return nil, p.err
	}
	var sum int64
	for stream.Receive() {
		sum += stream.Msg().Number
	}
	if stream.Err() != nil {
		return nil, stream.Err()
	}
	return connect.NewResponse(&pingv1.SumResponse{Sum: sum}), nil
}

func (p *pingServer) CountUp(
	_ context.Context,
	request *connect.Request[pingv1.CountUpRequest],
	stream *connect.ServerStream[pingv1.CountUpResponse],
) error {
	if p.err != nil {
		return p.err
	}
	for i := int64(1); i <= request.Msg.Number; i++ {
		if err := stream.Send(&pingv1.CountUpResponse{Number: i}); err != nil {
			return err
		}
	}
	return nil
}
