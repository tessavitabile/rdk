// Package gizmoapi implements the acme:component:gizmo API, a demonstraction API showcasing the available GRPC method types.
package gizmoapi

import (
	"context"
	"io"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils/rpc"

	pb "go.viam.com/rdk/examples/customresources/apis/proto/api/component/gizmo/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/utils/protoutils"
)

var Subtype = resource.NewSubtype(
	resource.Namespace("acme"),
	resource.ResourceTypeComponent,
	resource.SubtypeName("gizmo"),
)

// Named is a helper for getting the named Gizmo's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// FromRobot is a helper for getting the named Gizmo from the given Robot.
func FromRobot(r robot.Robot, name string) (Gizmo, error) {
	return robot.ResourceFromRobot[Gizmo](r, Named(name))
}

func init() {
	resource.RegisterSubtype(Subtype, resource.SubtypeRegistration[Gizmo]{
		// Reconfigurable, and contents of reconfwrapper.go are only needed for standalone (non-module) uses.
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterGizmoServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.GizmoService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name resource.Name, logger golog.Logger) (Gizmo, error) {
			return NewClientFromConn(conn, name, logger), nil
		},
	})

}

// Gizmo defines the Go interface for the component (should match the protobuf methods.)
type Gizmo interface {
	resource.Resource
	DoOne(ctx context.Context, arg1 string) (bool, error)
	DoOneClientStream(ctx context.Context, arg1 []string) (bool, error)
	DoOneServerStream(ctx context.Context, arg1 string) ([]bool, error)
	DoOneBiDiStream(ctx context.Context, arg1 []string) ([]bool, error)
	DoTwo(ctx context.Context, arg1 bool) (string, error)
}

// subtypeServer implements the Gizmo RPC service from gripper.proto.
type subtypeServer struct {
	pb.UnimplementedGizmoServiceServer
	coll resource.SubtypeCollection[Gizmo]
}

func NewRPCServiceServer(coll resource.SubtypeCollection[Gizmo]) interface{} {
	return &subtypeServer{coll: coll}
}

func (s *subtypeServer) DoOne(ctx context.Context, req *pb.DoOneRequest) (*pb.DoOneResponse, error) {
	g, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	resp, err := g.DoOne(ctx, req.Arg1)
	if err != nil {
		return nil, err
	}
	return &pb.DoOneResponse{Ret1: resp}, nil
}

func (s *subtypeServer) DoOneClientStream(server pb.GizmoService_DoOneClientStreamServer) error {
	var name string
	var args []string
	for {
		msg, err := server.Recv()
		if errors.Is(err, io.EOF) {
			break
		}

		args = append(args, msg.Arg1)
		if name == "" {
			name = msg.Name
			continue
		}
		if name != msg.Name {
			return errors.New("unexpected")
		}
	}
	g, err := s.coll.Resource(name)
	if err != nil {
		return err
	}
	resp, err := g.DoOneClientStream(server.Context(), args)
	if err != nil {
		return err
	}
	return server.SendAndClose(&pb.DoOneClientStreamResponse{Ret1: resp})
}

func (s *subtypeServer) DoOneServerStream(req *pb.DoOneServerStreamRequest, stream pb.GizmoService_DoOneServerStreamServer) error {
	g, err := s.coll.Resource(req.Name)
	if err != nil {
		return err
	}
	resp, err := g.DoOneServerStream(stream.Context(), req.Arg1)
	if err != nil {
		return err
	}
	for _, ret := range resp {
		if err := stream.Send(&pb.DoOneServerStreamResponse{
			Ret1: ret,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *subtypeServer) DoOneBiDiStream(server pb.GizmoService_DoOneBiDiStreamServer) error {
	var name string
	var args []string
	for {
		msg, err := server.Recv()
		if errors.Is(err, io.EOF) {
			break
		}

		args = append(args, msg.Arg1)
		if name == "" {
			name = msg.Name
			continue
		}
		if name != msg.Name {
			return errors.New("unexpected")
		}
	}
	g, err := s.coll.Resource(name)
	if err != nil {
		return err
	}
	resp, err := g.DoOneBiDiStream(server.Context(), args)
	if err != nil {
		return err
	}
	for _, respRet := range resp {
		if err := server.Send(&pb.DoOneBiDiStreamResponse{Ret1: respRet}); err != nil {
			return err
		}
	}
	return nil
}

func (s *subtypeServer) DoTwo(ctx context.Context, req *pb.DoTwoRequest) (*pb.DoTwoResponse, error) {
	g, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	resp, err := g.DoTwo(ctx, req.Arg1)
	if err != nil {
		return nil, err
	}
	return &pb.DoTwoResponse{Ret1: resp}, nil
}

func (s *subtypeServer) DoCommand(ctx context.Context, req *pb.DoCommandRequest) (*pb.DoCommandResponse, error) {
	g, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	resp, err := g.DoCommand(ctx, req.Command.AsMap())
	if err != nil {
		return nil, err
	}
	pbResp, err := protoutils.StructToStructPb(resp)
	if err != nil {
		return nil, err
	}
	return &pb.DoCommandResponse{Result: pbResp}, nil
}

func NewClientFromConn(conn rpc.ClientConn, name resource.Name, logger golog.Logger) Gizmo {
	sc := newSvcClientFromConn(conn, name, logger)
	return clientFromSvcClient(sc, name.ShortNameForClient())
}

func newSvcClientFromConn(conn rpc.ClientConn, name resource.Name, logger golog.Logger) *serviceClient {
	client := pb.NewGizmoServiceClient(conn)
	sc := &serviceClient{
		Named:  name.AsNamed(),
		client: client,
		logger: logger,
	}
	return sc
}

type serviceClient struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	client pb.GizmoServiceClient
	logger golog.Logger
}

// client is an gripper client.
type client struct {
	*serviceClient
	name string
}

func clientFromSvcClient(sc *serviceClient, name string) Gizmo {
	return &client{sc, name}
}

func (c *client) DoOne(ctx context.Context, arg1 string) (bool, error) {
	resp, err := c.client.DoOne(ctx, &pb.DoOneRequest{
		Name: c.name,
		Arg1: arg1,
	})
	if err != nil {
		return false, err
	}
	return resp.Ret1, nil
}

func (c *client) DoOneClientStream(ctx context.Context, arg1 []string) (bool, error) {
	client, err := c.client.DoOneClientStream(ctx)
	if err != nil {
		return false, err
	}
	for _, arg := range arg1 {
		if err := client.Send(&pb.DoOneClientStreamRequest{
			Name: c.name,
			Arg1: arg,
		}); err != nil {
			return false, err
		}
	}
	resp, err := client.CloseAndRecv()
	if err != nil {
		return false, err
	}
	return resp.Ret1, nil
}

func (c *client) DoOneServerStream(ctx context.Context, arg1 string) ([]bool, error) {
	resp, err := c.client.DoOneServerStream(ctx, &pb.DoOneServerStreamRequest{
		Name: c.name,
		Arg1: arg1,
	})
	if err != nil {
		return nil, err
	}
	var rets []bool
	for {
		resp, err := resp.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		rets = append(rets, resp.Ret1)
	}
	return rets, nil
}

func (c *client) DoOneBiDiStream(ctx context.Context, arg1 []string) ([]bool, error) {
	client, err := c.client.DoOneBiDiStream(ctx)
	if err != nil {
		return nil, err
	}
	for _, arg := range arg1 {
		if err := client.Send(&pb.DoOneBiDiStreamRequest{
			Name: c.name,
			Arg1: arg,
		}); err != nil {
			return nil, err
		}
	}
	if err := client.CloseSend(); err != nil {
		return nil, err
	}

	var rets []bool
	for {
		resp, err := client.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		rets = append(rets, resp.Ret1)
	}
	return rets, nil
}

func (c *client) DoTwo(ctx context.Context, arg1 bool) (string, error) {
	resp, err := c.client.DoTwo(ctx, &pb.DoTwoRequest{
		Name: c.name,
		Arg1: arg1,
	})
	if err != nil {
		return "", err
	}
	return resp.Ret1, nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	command, err := protoutils.StructToStructPb(cmd)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.DoCommand(ctx, &pb.DoCommandRequest{
		Name:    c.name,
		Command: command,
	})
	if err != nil {
		return nil, err
	}
	return resp.Result.AsMap(), nil
}
