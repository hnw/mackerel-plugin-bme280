package mpbme280

import (
	"flag"
	"fmt"
	"math"

	mp "github.com/mackerelio/go-mackerel-plugin"
	"gobot.io/x/gobot"
	"gobot.io/x/gobot/drivers/gpio"
	"gobot.io/x/gobot/drivers/i2c"
	"gobot.io/x/gobot/platforms/raspi"
)

// Bme280Plugin mackerel plugin
type Bme280Plugin struct {
	Prefix string
}

// MetricKeyPrefix interface for PluginWithPrefix
func (u Bme280Plugin) MetricKeyPrefix() string {
	if u.Prefix == "" {
		u.Prefix = "bme280"
	}
	return u.Prefix
}

// GraphDefinition interface for mackerelplugin
func (u Bme280Plugin) GraphDefinition() map[string]mp.Graphs {
	return map[string]mp.Graphs{
		"temperature.#": {
			Label: "Temperature (C)",
			Unit:  "float",
			Metrics: []mp.Metrics{
				{Name: "value", Label: "Temperature"},
			},
		},
		"pressure.#": {
			Label: "Pressure (hPa)",
			Unit:  "float",
			Metrics: []mp.Metrics{
				{Name: "value", Label: "Pressure"},
			},
		},
		"humidity.#": {
			Label: "Humidity (%)",
			Unit:  "float",
			Metrics: []mp.Metrics{
				{Name: "value", Label: "Humidity"},
			},
		},
		"abs_humidity.#": {
			Label: "Absolute Humidity (g/m^3)",
			Unit:  "float",
			Metrics: []mp.Metrics{
				{Name: "value", Label: "Absolute Humidity"},
			},
		},
		"raw_illum.#": {
			Label: "Illuminance (raw value)",
			Unit:  "integer",
			Metrics: []mp.Metrics{
				{Name: "broadband", Label: "Broadband light"},
				{Name: "infrared", Label: "Infrared light"},
			},
		},
		"illuminance.#": {
			Label: "Illuminance (lux)",
			Unit:  "integer",
			Metrics: []mp.Metrics{
				{Name: "value", Label: "Illuminance"},
			},
		},
	}
}

// FetchMetrics interface for mackerelplugin
func (u Bme280Plugin) FetchMetrics() (map[string]float64, error) {
	board := raspi.NewAdaptor()
	greenLed := gpio.NewLedDriver(board, "11")  // 11: GPIO 17
	yellowLed := gpio.NewLedDriver(board, "12") // 12: GPIO 18
	bme280 := i2c.NewBME280Driver(board)
	sht2x := i2c.NewSHT2xDriver(board)
	tsl2561 := i2c.NewTSL2561Driver(board, i2c.WithTSL2561Gain16X, i2c.WithAddress(0x29))
	metricsChan := make(chan map[string]float64, 1)
	yellowLed.Off()

	work := func() {
		greenLed.On()

		metrics := make(map[string]float64)

		t1, err := bme280.Temperature()
		if err == nil {
			metrics["temperature.BME280.value"] = float64(t1)
		}

		p1, err := bme280.Pressure()
		if err == nil {
			metrics["pressure.BME280.value"] = float64(p1 / 100.0)
		}

		rh1, err := bme280.Humidity()
		if err == nil {
			metrics["humidity.BME280.value"] = float64(rh1)
			metrics["abs_humidity.BME280.value"] = calcAbsoluteHumidity(float64(t1), float64(rh1))
		}

		t2, err := sht2x.Temperature()
		if err == nil {
			metrics["temperature.SHT2x.value"] = float64(t2)
		}

		rh2, err := sht2x.Humidity()
		if err == nil {
			metrics["humidity.SHT2x.value"] = float64(rh2)
			metrics["abs_humidity.SHT2x.value"] = calcAbsoluteHumidity(float64(t2), float64(rh2))
		}

		bb, ir, err := tsl2561.GetLuminocity()
		if err == nil {
			illum := tsl2561.CalculateLux(bb, ir)
			metrics["raw_illum.TSL2561.broadband"] = float64(bb)
			metrics["raw_illum.TSL2561.infrared"] = float64(ir)
			metrics["illuminance.TSL2561.value"] = float64(illum)
		}

		metricsChan <- metrics
	}

	robot := gobot.NewRobot("bme280bot",
		[]gobot.Connection{board},
		[]gobot.Device{bme280, sht2x, tsl2561},
		work,
	)

	err := robot.Start(false)
	if err != nil {
		greenLed.Off()
		yellowLed.On()
		return nil, fmt.Errorf("Failed to fetch metrics: %s", err)
	}
	metrics := <-metricsChan
	robot.Stop()
	return metrics, nil
}

// Do the plugin
func Do() {
	optPrefix := flag.String("metric-key-prefix", "bme280", "Metric key prefix")
	optTempfile := flag.String("tempfile", "", "Temp file name")
	flag.Parse()

	u := Bme280Plugin{
		Prefix: *optPrefix,
	}
	helper := mp.NewMackerelPlugin(u)
	helper.Tempfile = *optTempfile
	helper.Run()
}

// Calculate absolute humidity(g/m^3) from temperature(C) and relative humidity(%)
// This is based on Bolton's equation[1].
//
// [1] Bolton, D., The computation of equivalent potential temperature, Monthly Weather Review, 108, 1046-1053, 1980.
func calcAbsoluteHumidity(t float64, rh float64) (ah float64) {
	ah = 6.112 * math.Exp(17.67*t/(t+243.5)) * rh * 2.1674 / (273.15 + t)
	return
}
