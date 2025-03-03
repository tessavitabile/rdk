package adxl345

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

func nowNanosTest() uint64 {
	return uint64(time.Now().UnixNano())
}

func sendInterrupt(ctx context.Context, adxl movementsensor.MovementSensor, t *testing.T, interrupt board.DigitalInterrupt, key string) {
	interrupt.Tick(ctx, true, nowNanosTest())
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		readings, err := adxl.Readings(ctx, map[string]interface{}{})
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, readings[key], test.ShouldNotBeZeroValue)
	})
}

func TestInterrupts(t *testing.T) {
	ctx := context.Background()

	interrupt := &board.BasicDigitalInterrupt{}

	mockBoard := &inject.Board{}
	mockBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) { return interrupt, true }

	i2cHandle := &inject.I2CHandle{}
	i2cHandle.CloseFunc = func() error { return nil }
	i2cHandle.WriteByteDataFunc = func(context.Context, byte, byte) error { return nil }
	// The data returned from the readByteFunction is intended to signify which interrupts have gone off
	i2cHandle.ReadByteDataFunc = func(context.Context, byte) (byte, error) { return byte(1<<6 + 1<<2), nil }
	// i2cHandle.ReadBlockDataFunc gets called multiple times. The first time we need the first byte to be 0xE5 and the next
	// time we need 6 bytes. This return provides more data than necessary for the first call to the function but allows
	// both calls to it to work properly.
	i2cHandle.ReadBlockDataFunc = func(context.Context, byte, uint8) ([]byte, error) {
		return []byte{byte(0xE5), byte(0x1), byte(0x2), byte(0x3), byte(0x4), byte(0x5), byte(0x6)}, nil
	}

	i2c := &inject.I2C{}
	i2c.OpenHandleFunc = func(addr byte) (board.I2CHandle, error) { return i2cHandle, nil }

	mockBoard.I2CByNameFunc = func(name string) (board.I2C, bool) { return i2c, true }

	logger := golog.NewTestLogger(t)

	deps := resource.Dependencies{
		resource.NameFromSubtype(board.Subtype, "board"): mockBoard,
	}

	tap := &TapConfig{
		AccelerometerPin: 1,
		InterruptPin:     "int1",
	}

	ff := &FreeFallConfig{
		AccelerometerPin: 1,
		InterruptPin:     "int1",
	}

	cfg := resource.Config{
		Name:  "movementsensor",
		Model: modelName,
		API:   movementsensor.Subtype,
		ConvertedAttributes: &Config{
			BoardName: "board",
			I2cBus:    "bus",
			SingleTap: tap,
			FreeFall:  ff,
		},
	}

	t.Run("new adxl has interrupt counts set to 0", func(t *testing.T) {
		adxl, err := NewAdxl345(ctx, deps, cfg, logger)
		test.That(t, err, test.ShouldBeNil)

		readings, err := adxl.Readings(ctx, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readings["freefall_count"], test.ShouldEqual, 0)
		test.That(t, readings["single_tap_count"], test.ShouldEqual, 0)
	})

	t.Run("interrupts have been found correctly when both are configured to the same pin", func(t *testing.T) {
		adxl, err := NewAdxl345(ctx, deps, cfg, logger)
		test.That(t, err, test.ShouldBeNil)

		sendInterrupt(ctx, adxl, t, interrupt, "freefall_count")

		readings, err := adxl.Readings(ctx, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readings["freefall_count"], test.ShouldEqual, 1)
		test.That(t, readings["single_tap_count"], test.ShouldEqual, 1)
	})

	t.Run("interrupts have been found correctly only tap has been configured", func(t *testing.T) {
		cfg := resource.Config{
			Name:  "movementsensor",
			Model: modelName,
			API:   movementsensor.Subtype,
			ConvertedAttributes: &Config{
				BoardName: "board",
				I2cBus:    "bus",
				SingleTap: tap,
			},
		}

		adxl, err := NewAdxl345(ctx, deps, cfg, logger)
		test.That(t, err, test.ShouldBeNil)

		sendInterrupt(ctx, adxl, t, interrupt, "single_tap_count")

		readings, err := adxl.Readings(ctx, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readings["freefall_count"], test.ShouldEqual, 0)
		test.That(t, readings["single_tap_count"], test.ShouldEqual, 1)
	})

	t.Run("interrupts have been found correctly only freefall has been configured", func(t *testing.T) {
		cfg = resource.Config{
			Name:  "movementsensor",
			Model: modelName,
			API:   movementsensor.Subtype,
			ConvertedAttributes: &Config{
				BoardName: "board",
				I2cBus:    "bus",
				FreeFall:  ff,
			},
		}

		adxl, err := NewAdxl345(ctx, deps, cfg, logger)
		test.That(t, err, test.ShouldBeNil)

		sendInterrupt(ctx, adxl, t, interrupt, "freefall_count")

		readings, err := adxl.Readings(ctx, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readings["freefall_count"], test.ShouldEqual, 1)
		test.That(t, readings["single_tap_count"], test.ShouldEqual, 0)
	})
}

func TestReadInterrupts(t *testing.T) {
	ctx := context.Background()
	cancelContext, cancelFunc := context.WithCancel(ctx)
	i2cHandle := &inject.I2CHandle{}
	i2cHandle.CloseFunc = func() error { return nil }
	i2c := &inject.I2C{}
	i2c.OpenHandleFunc = func(addr byte) (board.I2CHandle, error) {
		return i2cHandle, nil
	}

	t.Run("increments tap and freefall counts when both interrupts have gone off", func(t *testing.T) {
		i2cHandle.ReadBlockDataFunc = func(context.Context, byte, uint8) ([]byte, error) {
			intSourceRegister := byte(1<<6) + byte(1<<2)
			return []byte{intSourceRegister}, nil
		}

		sensor := &adxl345{
			bus:               i2c,
			interruptsFound:   map[InterruptID]int{},
			cancelContext:     cancelContext,
			cancelFunc:        cancelFunc,
			interruptsEnabled: byte(1<<6 + 1<<2),
		}
		sensor.readInterrupts(sensor.cancelContext)
		test.That(t, sensor.interruptsFound[SingleTap], test.ShouldEqual, 1)
		test.That(t, sensor.interruptsFound[FreeFall], test.ShouldEqual, 1)
	})

	t.Run("increments freefall count only when freefall has gone off", func(t *testing.T) {
		i2cHandle.ReadBlockDataFunc = func(context.Context, byte, uint8) ([]byte, error) {
			intSourceRegister := byte(1 << 2)
			return []byte{intSourceRegister}, nil
		}

		sensor := &adxl345{
			bus:               i2c,
			interruptsFound:   map[InterruptID]int{},
			cancelContext:     cancelContext,
			cancelFunc:        cancelFunc,
			interruptsEnabled: byte(1<<6 + 1<<2),
		}
		sensor.readInterrupts(sensor.cancelContext)
		test.That(t, sensor.interruptsFound[SingleTap], test.ShouldEqual, 0)
		test.That(t, sensor.interruptsFound[FreeFall], test.ShouldEqual, 1)
	})

	t.Run("increments tap count only when only tap has gone off", func(t *testing.T) {
		i2cHandle.ReadBlockDataFunc = func(context.Context, byte, uint8) ([]byte, error) {
			intSourceRegister := byte(1 << 6)
			return []byte{intSourceRegister}, nil
		}

		sensor := &adxl345{
			bus:               i2c,
			interruptsFound:   map[InterruptID]int{},
			cancelContext:     cancelContext,
			cancelFunc:        cancelFunc,
			interruptsEnabled: byte(1<<6 + 1<<2),
		}
		sensor.readInterrupts(sensor.cancelContext)
		test.That(t, sensor.interruptsFound[SingleTap], test.ShouldEqual, 1)
		test.That(t, sensor.interruptsFound[FreeFall], test.ShouldEqual, 0)
	})

	t.Run("does not increment counts when neither interrupt has gone off", func(t *testing.T) {
		i2cHandle.ReadBlockDataFunc = func(context.Context, byte, uint8) ([]byte, error) {
			intSourceRegister := byte(0)
			return []byte{intSourceRegister}, nil
		}

		sensor := &adxl345{
			bus:               i2c,
			interruptsFound:   map[InterruptID]int{},
			cancelContext:     cancelContext,
			cancelFunc:        cancelFunc,
			interruptsEnabled: byte(1<<6 + 1<<2),
		}
		sensor.readInterrupts(sensor.cancelContext)
		test.That(t, sensor.interruptsFound[SingleTap], test.ShouldEqual, 0)
		test.That(t, sensor.interruptsFound[FreeFall], test.ShouldEqual, 0)
	})
}
