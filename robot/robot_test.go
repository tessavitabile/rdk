package robot_test

import (
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var (
	button1 = resource.NewName(resource.ResourceNamespaceRDK, resource.ResourceTypeComponent, resource.SubtypeName("button"), "arm1")

	armNames    = []resource.Name{arm.Named("arm1"), arm.Named("arm2"), arm.Named("remote:arm1")}
	buttonNames = []resource.Name{button1}
	sensorNames = []resource.Name{sensor.Named("sensor1")}
)

var hereRes = testutils.NewUnimplementedResource(generic.Named("here"))

func setupInjectRobot() *inject.Robot {
	arm3 := inject.NewArm("arm3")
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		if name.Name == "arm2" {
			return nil, resource.NewNotFoundError(name)
		}
		if name.Name == "arm3" {
			return arm3, nil
		}

		return hereRes, nil
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return testutils.ConcatResourceNames(
			armNames,
			buttonNames,
			sensorNames,
		)
	}

	return r
}

func TestAllResourcesByName(t *testing.T) {
	r := setupInjectRobot()

	resources := robot.AllResourcesByName(r, "arm1")
	test.That(t, resources, test.ShouldResemble, []resource.Resource{hereRes, hereRes})

	resources = robot.AllResourcesByName(r, "remote:arm1")
	test.That(t, resources, test.ShouldResemble, []resource.Resource{hereRes})

	test.That(t, func() { robot.AllResourcesByName(r, "arm2") }, test.ShouldPanic)

	resources = robot.AllResourcesByName(r, "sensor1")
	test.That(t, resources, test.ShouldResemble, []resource.Resource{hereRes})

	resources = robot.AllResourcesByName(r, "blah")
	test.That(t, resources, test.ShouldBeEmpty)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := robot.NamesBySubtype(r, gantry.Subtype)
	test.That(t, names, test.ShouldBeEmpty)

	names = robot.NamesBySubtype(r, sensor.Subtype)
	test.That(t, utils.NewStringSet(names...), test.ShouldResemble, utils.NewStringSet(testutils.ExtractNames(sensorNames...)...))

	names = robot.NamesBySubtype(r, arm.Subtype)
	test.That(t, utils.NewStringSet(names...), test.ShouldResemble, utils.NewStringSet(testutils.ExtractNames(armNames...)...))
}

func TestResourceFromRobot(t *testing.T) {
	r := setupInjectRobot()

	res, err := robot.ResourceFromRobot[arm.Arm](r, arm.Named("arm3"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	res, err = robot.ResourceFromRobot[arm.Arm](r, arm.Named("arm5"))
	test.That(t, err, test.ShouldBeError,
		resource.TypeError[arm.Arm](testutils.NewUnimplementedResource(generic.Named("foo"))))
	test.That(t, res, test.ShouldBeNil)

	res, err = robot.ResourceFromRobot[arm.Arm](r, arm.Named("arm2"))
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(arm.Named("arm2")))
	test.That(t, res, test.ShouldBeNil)
}
