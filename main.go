package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/characteristic"
	"github.com/caarlos0/env/v10"
	"github.com/caarlos0/go-solarman"
	"github.com/charmbracelet/log"
)

type Config struct {
	AppID     string `env:"APP_ID,required"`
	AppSecret string `env:"APP_SECRET,required"`
	Email     string `env:"EMAIL,required"`
	Password  string `env:"PASSWORD,required"`
}

func main() {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		log.Fatal("failed to parse config", "err", err)
	}

	fs := hap.NewFsStore("./db")

	cli, err := solarman.New(
		cfg.AppID,
		cfg.AppSecret,
		cfg.Email,
		cfg.Password,
	)
	if err != nil {
		log.Fatal("failed to create client", "err", err)
	}

	inverterDeviceID, err := findInverter(cli)
	if err != nil {
		log.Fatal("failed to find inverter", "err", err)
	}

	inverter := NewInverterSensor(accessory.Info{
		Name: "Solarman Inverter",
	})

	updateSensors := func() {
		data, err := cli.CurrentData(inverterDeviceID)
		if err != nil {
			log.Error("failed to get data", "err", err)
			return
		}

		if !data.Success {
			log.Error("API error", "msg", data.Msg, "code", data.Code)
			return
		}

		temp := get(data, "T_AC_RDT1")
		output := get(data, "APo_t1")
		rated := get(data, "Pr1")
		log.Info("sensors", "temp", temp, "output", output, "rated", rated)

		inverter.Temperature.CurrentTemperature.SetValue(temp)

		if output > 0 {
			_ = inverter.Battery.ChargingState.SetValue(characteristic.ChargingStateCharging)
		} else {
			_ = inverter.Battery.ChargingState.SetValue(characteristic.ChargingStateNotCharging)
		}
		if rated > 0 {
			_ = inverter.Battery.BatteryLevel.SetValue(
				int(output*100) / int(rated),
			)
		}

		inverter.Light.CurrentAmbientLightLevel.SetMaxValue(rated)
		inverter.Light.CurrentAmbientLightLevel.SetValue(output)
	}

	go func() {
		updateSensors()
		tick := time.NewTicker(time.Minute * 15)
		for range tick.C {
			updateSensors()
		}
	}()

	server, err := hap.NewServer(fs, inverter.A)
	if err != nil {
		log.Fatal("fail to start server", "error", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-c
		log.Info("stopping server...")
		signal.Stop(c)
		cancel()
	}()

	log.Info("starting server...")
	if err := server.ListenAndServe(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("failed to close server", "err", err)
	}
}

func findInverter(cli *solarman.Client) (int, error) {
	stations, err := cli.Stations()
	if err != nil {
		return 0, err
	}

	for _, station := range stations {
		log.Info("found station", "id", station.ID, "name", station.Name)
		devices, err := cli.StationDevices(station.ID)
		if err != nil {
			log.Error("failed to list devices", "station", station.ID, "err", err)
			continue
		}
		for _, device := range devices {
			log.Info("found device", "id", device.DeviceID, "sn", device.DeviceSn, "type", device.DeviceType)
			if device.DeviceType == "INVERTER" {
				return device.DeviceID, nil
			}
		}
	}

	return 0, errors.New("no inverter found")
}

func get(data solarman.CurrentData, key string) float64 {
	for _, s := range data.DataList {
		if s.Key == key {
			f, err := strconv.ParseFloat(s.Value, 64)
			if err != nil {
				return 0
			}
			return f
		}
	}
	return 0
}
