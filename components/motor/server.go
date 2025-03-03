// Package motor contains a gRPC based motor service server
package motor

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/motor/v1"

	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

type subtypeServer struct {
	pb.UnimplementedMotorServiceServer
	coll resource.SubtypeCollection[Motor]
}

// NewRPCServiceServer constructs a motor gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.SubtypeCollection[Motor]) interface{} {
	return &subtypeServer{coll: coll}
}

// SetPower sets the percentage of power the motor of the underlying robot should employ between 0-1.
func (server *subtypeServer) SetPower(
	ctx context.Context,
	req *pb.SetPowerRequest,
) (*pb.SetPowerResponse, error) {
	motorName := req.GetName()
	motor, err := server.coll.Resource(motorName)
	if err != nil {
		return nil, err
	}
	return &pb.SetPowerResponse{}, motor.SetPower(ctx, req.GetPowerPct(), req.Extra.AsMap())
}

// GoFor requests the motor of the underlying robot to go for a certain amount based off
// the request.
func (server *subtypeServer) GoFor(
	ctx context.Context,
	req *pb.GoForRequest,
) (*pb.GoForResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.GetName())
	motorName := req.GetName()
	motor, err := server.coll.Resource(motorName)
	if err != nil {
		return nil, err
	}

	return &pb.GoForResponse{}, motor.GoFor(ctx, req.GetRpm(), req.GetRevolutions(), req.Extra.AsMap())
}

// GetPosition reports the position of the motor of the underlying robot
// based on its encoder. If it's not supported, the returned data is undefined.
// The unit returned is the number of revolutions which is intended to be fed
// back into calls of GoFor.
func (server *subtypeServer) GetPosition(
	ctx context.Context,
	req *pb.GetPositionRequest,
) (*pb.GetPositionResponse, error) {
	motorName := req.GetName()
	motor, err := server.coll.Resource(motorName)
	if err != nil {
		return nil, err
	}

	pos, err := motor.Position(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetPositionResponse{Position: pos}, nil
}

// GetProperties returns a message of booleans indicating which optional features the robot's motor supports.
func (server *subtypeServer) GetProperties(
	ctx context.Context,
	req *pb.GetPropertiesRequest,
) (*pb.GetPropertiesResponse, error) {
	motorName := req.GetName()
	motor, err := server.coll.Resource(motorName)
	if err != nil {
		return nil, err
	}
	features, err := motor.Properties(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return FeatureMapToProtoResponse(features)
}

// Stop turns the motor of the underlying robot off.
func (server *subtypeServer) Stop(
	ctx context.Context,
	req *pb.StopRequest,
) (*pb.StopResponse, error) {
	motorName := req.GetName()
	motor, err := server.coll.Resource(motorName)
	if err != nil {
		return nil, err
	}

	return &pb.StopResponse{}, motor.Stop(ctx, req.Extra.AsMap())
}

// IsPowered returns whether or not the motor of the underlying robot is currently on.
func (server *subtypeServer) IsPowered(
	ctx context.Context,
	req *pb.IsPoweredRequest,
) (*pb.IsPoweredResponse, error) {
	motorName := req.GetName()
	motor, err := server.coll.Resource(motorName)
	if err != nil {
		return nil, err
	}

	isOn, powerPct, err := motor.IsPowered(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.IsPoweredResponse{IsOn: isOn, PowerPct: powerPct}, nil
}

// GoTo requests the motor of the underlying robot to go a specific position.
func (server *subtypeServer) GoTo(
	ctx context.Context,
	req *pb.GoToRequest,
) (*pb.GoToResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.GetName())
	motorName := req.GetName()
	motor, err := server.coll.Resource(motorName)
	if err != nil {
		return nil, err
	}

	return &pb.GoToResponse{}, motor.GoTo(ctx, req.GetRpm(), req.GetPositionRevolutions(), req.Extra.AsMap())
}

// ResetZeroPosition sets the current position of the motor specified by the request
// (adjusted by a given offset) to be its new zero position.
func (server *subtypeServer) ResetZeroPosition(
	ctx context.Context,
	req *pb.ResetZeroPositionRequest,
) (*pb.ResetZeroPositionResponse, error) {
	motorName := req.GetName()
	motor, err := server.coll.Resource(motorName)
	if err != nil {
		return nil, err
	}

	return &pb.ResetZeroPositionResponse{}, motor.ResetZeroPosition(ctx, req.GetOffset(), req.Extra.AsMap())
}

// IsMoving queries of a component is in motion.
func (server *subtypeServer) IsMoving(ctx context.Context, req *pb.IsMovingRequest) (*pb.IsMovingResponse, error) {
	motor, err := server.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	moving, err := motor.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.IsMovingResponse{IsMoving: moving}, nil
}

// DoCommand receives arbitrary commands.
func (server *subtypeServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	motor, err := server.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, motor, req)
}
