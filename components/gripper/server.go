// Package gripper contains a gRPC based gripper service server.
package gripper

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/gripper/v1"

	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// subtypeServer implements the GripperService from gripper.proto.
type subtypeServer struct {
	pb.UnimplementedGripperServiceServer
	coll resource.SubtypeCollection[Gripper]
}

// NewRPCServiceServer constructs an gripper gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.SubtypeCollection[Gripper]) interface{} {
	return &subtypeServer{coll: coll}
}

// Open opens a gripper of the underlying robot.
func (s *subtypeServer) Open(ctx context.Context, req *pb.OpenRequest) (*pb.OpenResponse, error) {
	gripper, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.OpenResponse{}, gripper.Open(ctx, req.Extra.AsMap())
}

// Grab requests a gripper of the underlying robot to grab.
func (s *subtypeServer) Grab(ctx context.Context, req *pb.GrabRequest) (*pb.GrabResponse, error) {
	gripper, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	success, err := gripper.Grab(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GrabResponse{Success: success}, nil
}

// Stop stops the gripper specified.
func (s *subtypeServer) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	gripper, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.StopResponse{}, gripper.Stop(ctx, req.Extra.AsMap())
}

// IsMoving queries of a component is in motion.
func (s *subtypeServer) IsMoving(ctx context.Context, req *pb.IsMovingRequest) (*pb.IsMovingResponse, error) {
	gripper, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	moving, err := gripper.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.IsMovingResponse{IsMoving: moving}, nil
}

// DoCommand receives arbitrary commands.
func (s *subtypeServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	gripper, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, gripper, req)
}
