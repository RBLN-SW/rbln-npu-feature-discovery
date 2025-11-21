package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/rebellions-sw/rbln-npu-feature-discovery/pkg/rblnservicespb"
)

type Client struct {
	conn   *grpc.ClientConn
	client rblnservicespb.RBLNServicesClient
}

func NewClient(ctx context.Context, endpoint string) (*Client, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	//nolint:staticcheck // keep WithBlock until gRPC client migration finishes
	conn, err := grpc.DialContext(
		dialCtx,
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(), //nolint:staticcheck // keep withblock until grpc client migration is finished
	)
	if err != nil {
		return nil, fmt.Errorf("failed to dial rbln-daemon %s: %w", endpoint, err)
	}

	return &Client{
		conn:   conn,
		client: rblnservicespb.NewRBLNServicesClient(conn),
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) ServiceableDevices(ctx context.Context) ([]*rblnservicespb.Device, error) {
	stream, err := c.client.GetServiceableDeviceList(ctx, &rblnservicespb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("failed to GetServiceableDeviceList RPC: %w", err)
	}

	var devices []*rblnservicespb.Device
	for {
		d, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("failed to receive device: %w", err)
		}
		devices = append(devices, d)
	}
	return devices, nil
}

func (c *Client) Version(ctx context.Context, device *rblnservicespb.Device) (*rblnservicespb.VersionInfo, error) {
	resp, err := c.client.GetVersion(ctx, device)
	if err != nil {
		return nil, fmt.Errorf("failed to GetVersion RPC: %w", err)
	}
	return resp, nil
}
