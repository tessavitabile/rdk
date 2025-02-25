// Package datamanager contains a gRPC based datamanager service server
package datamanager

import (
	"context"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/service/datamanager/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client implements DataManagerServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.DataManagerServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name resource.Name, logger golog.Logger) (Service, error) {
	grpcClient := pb.NewDataManagerServiceClient(conn)
	c := &client{
		Named:  name.AsNamed(),
		name:   name.ShortNameForClient(),
		client: grpcClient,
		logger: logger,
	}
	return c, nil
}

func (c *client) Sync(ctx context.Context, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.Sync(ctx, &pb.SyncRequest{Name: c.name, Extra: ext})
	if err != nil {
		return err
	}
	return nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}
