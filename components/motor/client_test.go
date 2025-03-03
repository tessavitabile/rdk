package motor_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/motor"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	workingMotor := &inject.Motor{}
	failingMotor := &inject.Motor{}

	var actualExtra map[string]interface{}
	var actualPowerPct float64

	workingMotor.SetPowerFunc = func(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
		actualExtra = extra
		actualPowerPct = powerPct
		return nil
	}
	workingMotor.GoForFunc = func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}
	workingMotor.GoToFunc = func(ctx context.Context, rpm, position float64, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}
	workingMotor.ResetZeroPositionFunc = func(ctx context.Context, offset float64, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}
	workingMotor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		actualExtra = extra
		return 42.0, nil
	}
	workingMotor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
		actualExtra = extra
		return map[motor.Feature]bool{
			motor.PositionReporting: true,
		}, nil
	}
	workingMotor.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		actualExtra = extra
		return nil
	}
	workingMotor.IsPoweredFunc = func(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
		actualExtra = extra
		return true, actualPowerPct, nil
	}

	failingMotor.SetPowerFunc = func(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
		return errors.New("set power failed")
	}
	failingMotor.GoForFunc = func(ctx context.Context, rpm, rotations float64, extra map[string]interface{}) error {
		return errors.New("go for failed")
	}
	failingMotor.GoToFunc = func(ctx context.Context, rpm, position float64, extra map[string]interface{}) error {
		return errors.New("go to failed")
	}
	failingMotor.ResetZeroPositionFunc = func(ctx context.Context, offset float64, extra map[string]interface{}) error {
		return errors.New("set to zero failed")
	}
	failingMotor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 0, errors.New("position unavailable")
	}
	failingMotor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
		return nil, errors.New("supported features unavailable")
	}
	failingMotor.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
		return errors.New("stop failed")
	}
	failingMotor.IsPoweredFunc = func(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
		return false, 0.0, errors.New("is on unavailable")
	}

	resourceMap := map[resource.Name]motor.Motor{
		motor.Named(testMotorName): workingMotor,
		motor.Named(failMotorName): failingMotor,
	}
	motorSvc, err := resource.NewSubtypeCollection(motor.Subtype, resourceMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype, ok, err := resource.LookupSubtypeRegistration[motor.Motor](motor.Subtype)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceSubtype.RegisterRPCService(context.Background(), rpcServer, motorSvc), test.ShouldBeNil)

	workingMotor.DoFunc = testutils.EchoFunc

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
	workingMotorClient, err := motor.NewClientFromConn(context.Background(), conn, motor.Named(testMotorName), logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("client tests for working motor", func(t *testing.T) {
		// DoCommand
		resp, err := workingMotorClient.DoCommand(context.Background(), testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp["command"], test.ShouldEqual, testutils.TestCommand["command"])
		test.That(t, resp["data"], test.ShouldEqual, testutils.TestCommand["data"])

		err = workingMotorClient.SetPower(context.Background(), 42.0, nil)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorClient.GoFor(context.Background(), 42.0, 42.0, nil)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorClient.GoTo(context.Background(), 42.0, 42.0, nil)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorClient.ResetZeroPosition(context.Background(), 0.5, nil)
		test.That(t, err, test.ShouldBeNil)

		pos, err := workingMotorClient.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 42.0)

		features, err := workingMotorClient.Properties(context.Background(), nil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorClient.Stop(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		isOn, powerPct, err := workingMotorClient.IsPowered(
			context.Background(),
			map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}})
		test.That(t, isOn, test.ShouldBeTrue)
		test.That(t, powerPct, test.ShouldEqual, 42.0)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, actualExtra, test.ShouldResemble, map[string]interface{}{"foo": "bar", "baz": []interface{}{1., 2., 3.}})

		test.That(t, workingMotorClient.Close(context.Background()), test.ShouldBeNil)

		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	conn, err = viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	failingMotorClient, err := motor.NewClientFromConn(context.Background(), conn, motor.Named(failMotorName), logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("client tests for failing motor", func(t *testing.T) {
		err := failingMotorClient.GoTo(context.Background(), 42.0, 42.0, nil)
		test.That(t, err, test.ShouldNotBeNil)

		err = failingMotorClient.ResetZeroPosition(context.Background(), 0.5, nil)
		test.That(t, err, test.ShouldNotBeNil)

		pos, err := failingMotorClient.Position(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, pos, test.ShouldEqual, 0.0)

		err = failingMotorClient.SetPower(context.Background(), 42.0, nil)
		test.That(t, err, test.ShouldNotBeNil)

		err = failingMotorClient.GoFor(context.Background(), 42.0, 42.0, nil)
		test.That(t, err, test.ShouldNotBeNil)

		features, err := failingMotorClient.Properties(context.Background(), nil)
		test.That(t, features, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)

		isOn, powerPct, err := failingMotorClient.IsPowered(context.Background(), nil)
		test.That(t, isOn, test.ShouldBeFalse)
		test.That(t, powerPct, test.ShouldEqual, 0.0)
		test.That(t, err, test.ShouldNotBeNil)

		err = failingMotorClient.Stop(context.Background(), nil)
		test.That(t, err, test.ShouldNotBeNil)

		test.That(t, failingMotorClient.Close(context.Background()), test.ShouldBeNil)
	})

	t.Run("dialed client tests for working motor", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		workingMotorDialedClient, err := motor.NewClientFromConn(context.Background(), conn, motor.Named(testMotorName), logger)
		test.That(t, err, test.ShouldBeNil)

		pos, err := workingMotorDialedClient.Position(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos, test.ShouldEqual, 42.0)

		features, err := workingMotorDialedClient.Properties(context.Background(), nil)
		test.That(t, features[motor.PositionReporting], test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorDialedClient.GoTo(context.Background(), 42.0, 42.0, nil)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorDialedClient.ResetZeroPosition(context.Background(), 0.5, nil)
		test.That(t, err, test.ShouldBeNil)

		err = workingMotorDialedClient.Stop(context.Background(), nil)
		test.That(t, err, test.ShouldBeNil)

		isOn, powerPct, err := workingMotorDialedClient.IsPowered(context.Background(), nil)
		test.That(t, isOn, test.ShouldBeTrue)
		test.That(t, powerPct, test.ShouldEqual, 42.0)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, workingMotorDialedClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})

	t.Run("dialed client tests for failing motor", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		failingMotorDialedClient, err := motor.NewClientFromConn(context.Background(), conn, motor.Named(failMotorName), logger)
		test.That(t, err, test.ShouldBeNil)

		err = failingMotorDialedClient.SetPower(context.Background(), 39.2, nil)
		test.That(t, err, test.ShouldNotBeNil)

		features, err := failingMotorDialedClient.Properties(context.Background(), nil)
		test.That(t, features, test.ShouldBeNil)
		test.That(t, err, test.ShouldNotBeNil)

		isOn, powerPct, err := failingMotorDialedClient.IsPowered(context.Background(), nil)
		test.That(t, isOn, test.ShouldBeFalse)
		test.That(t, powerPct, test.ShouldEqual, 0.0)
		test.That(t, err, test.ShouldNotBeNil)

		test.That(t, failingMotorDialedClient.Close(context.Background()), test.ShouldBeNil)
		test.That(t, conn.Close(), test.ShouldBeNil)
	})
	test.That(t, conn.Close(), test.ShouldBeNil)
}
