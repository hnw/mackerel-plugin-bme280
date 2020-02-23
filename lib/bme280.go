package mpbme280

import (
	"flag"
	"fmt"

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
		"temperature": {
			Label: "Temperature (C)",
			Unit:  "float",
			Metrics: []mp.Metrics{
				{Name: "temperature", Label: "Temperature"},
			},
		},
		"pressure": {
			Label: "Pressure (hPa)",
			Unit:  "float",
			Metrics: []mp.Metrics{
				{Name: "pressure", Label: "Pressure"},
			},
		},
		"humidity": {
			Label: "Humidity (%)",
			Unit:  "float",
			Metrics: []mp.Metrics{
				{Name: "humidity", Label: "Humidity"},
			},
		},
		"raw_illum": {
			Label: "Illuminance (raw value)",
			Unit:  "integer",
			Metrics: []mp.Metrics{
				{Name: "broadband", Label: "Broadband light"},
				{Name: "infrared", Label: "Infrared light"},
			},
		},
		"illuminance": {
			Label: "Illuminance (lux)",
			Unit:  "integer",
			Metrics: []mp.Metrics{
				{Name: "illuminance", Label: "Illuminance"},
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
	tsl2561 := i2c.NewTSL2561Driver(board, i2c.WithTSL2561Gain16X, i2c.WithAddress(0x29))
	metricsChan := make(chan map[string]float64, 1)
	yellowLed.Off()

	work := func() {
		greenLed.On()

		metrics := make(map[string]float64)

		t, err := bme280.Temperature()
		if err == nil {
			metrics["temperature"] = float64(t)
		}

		p, err := bme280.Pressure()
		if err == nil {
			metrics["pressure"] = float64(p / 100.0)
		}

		h, err := bme280.Humidity()
		if err == nil {
			metrics["humidity"] = float64(h)
		}

		bb, ir, err := tsl2561.GetLuminocity()
		if err == nil {
			illum := tsl2561.CalculateLux(bb, ir)
			metrics["broadband"] = float64(bb)
			metrics["infrared"] = float64(ir)
			metrics["illuminance"] = float64(illum)
		}

		metricsChan <- metrics
	}

	robot := gobot.NewRobot("bme280bot",
		[]gobot.Connection{board},
		[]gobot.Device{bme280, tsl2561},
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
