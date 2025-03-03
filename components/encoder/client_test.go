package encoder_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/encoder/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/encoder"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testEncoderName    = "encoder1"
	testEncoderName2   = "encoder2"
	failEncoderName    = "encoder3"
	fakeEncoderName    = "encoder4"
	missingEncoderName = "encoder5"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	workingEncoder := &inject.Encoder{}
	failingEncoder := &inject.Encoder{}

	var actualExtra map[string]interface{}

	workingEncoder.ResetPositionFunc = func(ctx context.Context, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}
	workingEncoder.GetPositionFunc = func(
		ctx context.Context,
		positionType encoder.PositionType,
		extra map[string]interface{},
	) (float64, encoder.PositionType, error) {
		actualExtra = extra
		return 42.0, encoder.PositionTypeUnspecified, nil
	}
	workingEncoder.GetPropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (map[encoder.Feature]bool, error) {
		actualExtra = extra
		return map[encoder.Feature]bool{
			encoder.TicksCountSupported:   true,
			encoder.AngleDegreesSupported: false,
		}, nil
	}

	failingEncoder.ResetPositionFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return errors.New("set to zero failed")
	}
	failingEncoder.GetPositionFunc = func(
		ctx context.Context,
		positionType encoder.PositionType,
		extra map[string]interface{},
	) (float64, encoder.PositionType, error) {
		return 0, encoder.PositionTypeUnspecified, errors.New("position unavailable")
	}
	failingEncoder.GetPropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (map[encoder.Feature]bool, error) {
		return nil, errors.New("get properties failed")
	}

	resourceMap := map[resource.Name]encoder.Encoder{
		encoder.Named(testEncoderName): workingEncoder,
		encoder.Named(failEncoderName): failingEncoder,
	}
	encoderSvc, err := resource.NewSubtypeCollection(encoder.Subtype, resourceMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype, ok, err := resource.LookupSubtypeRegistration[encoder.Encoder](encoder.Subtype)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceSubtype.RegisterRPCService(context.Background(), rpcServer, encoderSvc), test.ShouldBeNil)

	workingEncoder.DoFunc = testutils.EchoFunc

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("Failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := viamgrpc.Dial(cancelCtx, listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	workingEncoderClient := encoder.NewClientFromConn(context.Background(), conn, encoder.Named(testEncoderName), logger)

	t.Run("client tests for working encoder", func(t *testing.T) {
		// DoCommand
		resp, err := workingEncoderClient.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		err = workingEncoderClient.ResetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		pos, positionType, err := workingEncoderClient.GetPosition(
			context.Background(),
			encoder.PositionTypeUnspecified,
			map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 42.0)
		test.That(t, positionType, test.ShouldEqual, pb.PositionType_POSITION_TYPE_UNSPECIFIED)

		test.That(t, actualExtra, test.ShouldResemble, map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}})

		test.That(t, workingEncoderClient.Close(context.Background()), test.ShouldBeNil)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	conn, err = viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	failingEncoderClient := encoder.NewClientFromConn(context.Background(), conn, encoder.Named(failEncoderName), logger)

	t.Run("client tests for failing encoder", func(t *testing.T) {
		err = failingEncoderClient.ResetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)

		pos, _, err := failingEncoderClient.GetPosition(context.Background(), encoder.PositionTypeUnspecified, nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, pos, test.ShouldEqual, 0.0)

		test.That(t, failingEncoderClient.Close(context.Background()), test.ShouldBeNil)
	})

	t.Run("dialed client tests for working encoder", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		workingEncoderDialedClient := encoder.NewClientFromConn(context.Background(), conn, encoder.Named(testEncoderName), logger)

		pos, _, err := workingEncoderDialedClient.GetPosition(context.Background(), encoder.PositionTypeUnspecified, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 42.0)

		err = workingEncoderDialedClient.ResetPosition(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, workingEncoderDialedClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("dialed client tests for failing encoder", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		failingEncoderDialedClient := encoder.NewClientFromConn(context.Background(), conn, encoder.Named(failEncoderName), logger)

		test.That(t, failingEncoderDialedClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	test.That(t, conn.Close(), test.ShouldBeNil)
}
