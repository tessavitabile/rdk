package encoder

import (
	"context"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/encoder/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

type subtypeServer struct {
	pb.UnimplementedEncoderServiceServer
	coll resource.SubtypeCollection[Encoder]
}

// NewRPCServiceServer constructs an Encoder gRPC service subtypeServer.
func NewRPCServiceServer(coll resource.SubtypeCollection[Encoder]) interface{} {
	return &subtypeServer{coll: coll}
}

// GetPosition returns the current position in terms of ticks or
// degrees, and whether it is a relative or absolute position.
func (s *subtypeServer) GetPosition(
	ctx context.Context,
	req *pb.GetPositionRequest,
) (*pb.GetPositionResponse, error) {
	enc, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	position, positionType, err := enc.GetPosition(ctx, ToEncoderPositionType(req.PositionType), req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetPositionResponse{
		Value:        float32(position),
		PositionType: ToProtoPositionType(positionType),
	}, nil
}

// ResetPosition sets the current position of the encoder
// specified by the request to be its new zero position.
func (s *subtypeServer) ResetPosition(
	ctx context.Context,
	req *pb.ResetPositionRequest,
) (*pb.ResetPositionResponse, error) {
	encName := req.GetName()
	enc, err := s.coll.Resource(encName)
	if err != nil {
		return nil, errors.Errorf("no encoder (%s) found", encName)
	}

	return &pb.ResetPositionResponse{}, enc.ResetPosition(ctx, req.Extra.AsMap())
}

// GetProperties returns a message of booleans indicating which optional features the robot's encoder supports.
func (s *subtypeServer) GetProperties(
	ctx context.Context,
	req *pb.GetPropertiesRequest,
) (*pb.GetPropertiesResponse, error) {
	encoderName := req.GetName()
	enc, err := s.coll.Resource(encoderName)
	if err != nil {
		return nil, errors.Errorf("no encoder (%s) found", encoderName)
	}
	features, err := enc.GetProperties(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return FeatureMapToProtoResponse(features)
}

// DoCommand receives arbitrary commands.
func (s *subtypeServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	enc, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, enc, req)
}
