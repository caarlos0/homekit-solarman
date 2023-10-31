package main

import (
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/service"
)

type Inverter struct {
	*accessory.A
	Temperature *service.TemperatureSensor
	Battery     *service.BatteryService
	Light       *service.LightSensor
}

func NewInverterSensor(info accessory.Info) *Inverter {
	a := Inverter{}
	a.A = accessory.New(info, accessory.TypeSensor)

	a.Temperature = service.NewTemperatureSensor()
	a.AddS(a.Temperature.S)

	a.Battery = service.NewBatteryService()
	a.AddS(a.Battery.S)

	a.Light = service.NewLightSensor()
	a.AddS(a.Light.S)

	return &a
}
