package fake

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/encoder/fake"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/resource"
)

func TestMotorInit(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	enc, err := fake.NewEncoder(context.Background(), resource.Config{
		ConvertedAttributes: &fake.Config{},
	})
	test.That(t, err, test.ShouldBeNil)
	m := &Motor{
		Encoder:           enc.(fake.Encoder),
		Logger:            logger,
		PositionReporting: true,
		MaxRPM:            60,
		TicksPerRotation:  1,
	}

	pos, err := m.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 0)

	featureMap, err := m.Properties(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, featureMap[motor.PositionReporting], test.ShouldBeTrue)
}

func TestGoFor(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	enc, err := fake.NewEncoder(context.Background(), resource.Config{
		ConvertedAttributes: &fake.Config{},
	})
	test.That(t, err, test.ShouldBeNil)
	m := &Motor{
		Encoder:           enc.(fake.Encoder),
		Logger:            logger,
		PositionReporting: true,
		MaxRPM:            60,
		TicksPerRotation:  1,
	}

	err = m.GoFor(ctx, 0, 1, nil)
	test.That(t, err, test.ShouldBeError, motor.NewZeroRPMError())
	err = m.GoFor(ctx, 60, 1, nil)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		pos, err := m.Position(ctx, nil)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, pos, test.ShouldEqual, 1)
	})
}

func TestGoTo(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	enc, err := fake.NewEncoder(context.Background(), resource.Config{
		ConvertedAttributes: &fake.Config{},
	})
	test.That(t, err, test.ShouldBeNil)
	m := &Motor{
		Encoder:           enc.(fake.Encoder),
		Logger:            logger,
		PositionReporting: true,
		MaxRPM:            60,
		TicksPerRotation:  1,
	}

	err = m.GoTo(ctx, 60, 1, nil)
	test.That(t, err, test.ShouldBeNil)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		pos, err := m.Position(ctx, nil)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, pos, test.ShouldEqual, 1)
	})
}

func TestGoTillStop(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	enc, err := fake.NewEncoder(context.Background(), resource.Config{
		ConvertedAttributes: &fake.Config{},
	})
	test.That(t, err, test.ShouldBeNil)
	m := &Motor{
		Named:             motor.Named("foo").AsNamed(),
		Encoder:           enc.(fake.Encoder),
		Logger:            logger,
		PositionReporting: true,
		MaxRPM:            60,
		TicksPerRotation:  1,
	}

	err = m.GoTillStop(ctx, 0, func(ctx context.Context) bool { return false })
	test.That(t, err, test.ShouldNotBeNil)
}

func TestResetZeroPosition(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	enc, err := fake.NewEncoder(context.Background(), resource.Config{
		ConvertedAttributes: &fake.Config{},
	})
	test.That(t, err, test.ShouldBeNil)
	m := &Motor{
		Encoder:           enc.(fake.Encoder),
		Logger:            logger,
		PositionReporting: true,
		MaxRPM:            60,
		TicksPerRotation:  1,
	}

	err = m.ResetZeroPosition(ctx, 0, nil)
	test.That(t, err, test.ShouldBeNil)

	pos, err := m.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pos, test.ShouldEqual, 0)
}

func TestPower(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	enc, err := fake.NewEncoder(context.Background(), resource.Config{
		ConvertedAttributes: &fake.Config{},
	})
	test.That(t, err, test.ShouldBeNil)
	m := &Motor{
		Encoder:           enc.(fake.Encoder),
		Logger:            logger,
		PositionReporting: true,
		MaxRPM:            60,
		TicksPerRotation:  1,
	}

	err = m.SetPower(ctx, 1.0, nil)
	test.That(t, err, test.ShouldBeNil)

	powerPct := m.PowerPct()
	test.That(t, powerPct, test.ShouldEqual, 1.0)

	dir := m.Direction()
	test.That(t, dir, test.ShouldEqual, 1)

	isPowered, powerPctReported, err := m.IsPowered(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, isPowered, test.ShouldEqual, true)
	test.That(t, powerPctReported, test.ShouldEqual, powerPct)

	isMoving, err := m.IsMoving(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, isMoving, test.ShouldEqual, true)

	err = m.Stop(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	powerPct = m.PowerPct()
	test.That(t, powerPct, test.ShouldEqual, 0.0)
}
