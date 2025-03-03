// Package builtin implements the default sensors service.
package builtin

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/internal"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/sensors"
)

func init() {
	resource.RegisterDefaultService(sensors.Subtype, resource.DefaultServiceModel, resource.Registration[sensors.Service, resource.NoNativeConfig]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (sensors.Service, error) {
			return NewBuiltIn(ctx, deps, conf, logger)
		},
		// NOTE(erd): this ideally would be a matcher on all resources that
		// contain a Readings proto method. However, these resource types
		// are heterogeneous and do not actually have this proto method.
		// We need to either 1. know in advance what kinds of resources are
		// sensors 2. use a heuristic to detect it or 3. encode it in proto.
		WeakDependencies: []internal.ResourceMatcher{internal.ComponentDependencyWildcardMatcher},
	})
}

// NewBuiltIn returns a new default sensor service for the given robot.
func NewBuiltIn(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (sensors.Service, error) {
	s := &builtIn{
		Named:   conf.ResourceName().AsNamed(),
		sensors: map[resource.Name]sensor.Sensor{},
		logger:  logger,
	}
	if err := s.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return s, nil
}

type builtIn struct {
	resource.Named
	resource.TriviallyCloseable
	mu      sync.RWMutex
	sensors map[resource.Name]sensor.Sensor
	logger  golog.Logger
}

// Sensors returns all sensors in the robot.
func (s *builtIn) Sensors(ctx context.Context, extra map[string]interface{}) ([]resource.Name, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]resource.Name, 0, len(s.sensors))
	for name := range s.sensors {
		names = append(names, name)
	}
	return names, nil
}

// Readings returns the readings of the resources specified.
func (s *builtIn) Readings(ctx context.Context, sensorNames []resource.Name, extra map[string]interface{}) ([]sensors.Readings, error) {
	s.mu.RLock()
	// make a copy of sensors and then unlock
	sensorsMap := make(map[resource.Name]sensor.Sensor, len(s.sensors))
	for name, sensor := range s.sensors {
		sensorsMap[name] = sensor
	}
	s.mu.RUnlock()

	// dedupe sensorNames
	deduped := make(map[resource.Name]struct{}, len(sensorNames))
	for _, val := range sensorNames {
		deduped[val] = struct{}{}
	}

	readings := make([]sensors.Readings, 0, len(deduped))
	for name := range deduped {
		sensor, ok := sensorsMap[name]
		if !ok {
			return nil, errors.Errorf("resource %q not a registered sensor", name)
		}
		reading, err := sensor.Readings(ctx, extra)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get reading from %q", name)
		}
		readings = append(readings, sensors.Readings{Name: name, Readings: reading})
	}
	return readings, nil
}

func (s *builtIn) Reconfigure(ctx context.Context, deps resource.Dependencies, _ resource.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sensors := map[resource.Name]sensor.Sensor{}
	for n, r := range deps {
		if sensor, ok := r.(sensor.Sensor); ok {
			sensors[n] = sensor
		}
	}
	s.sensors = sensors
	return nil
}
