package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	. "gopkg.in/check.v1"
)

func (s *TestSuite) TestParsePayload(c *C) {
	b, err := ioutil.ReadFile(filepath.Join("testdata", "example.json"))
	c.Assert(err, IsNil)

	var pl payload
	err = json.Unmarshal(b, &pl)
	c.Assert(err, IsNil)

	c.Assert(pl.Coordinator, Not(IsNil))

	c.Assert(pl.Coordinator.CoordinatorID, Equals, int64(20))
	c.Assert(pl.Coordinator.GSMCoverage, Equals, int64(26))
	c.Assert(pl.Coordinator.BatteryVoltage, Equals, int64(166))
	c.Assert(pl.Coordinator.Uptime, Equals, int64(1890037))
	c.Assert(pl.Coordinator.FirstOverflow, Equals, int64(0))
	c.Assert(pl.Coordinator.Tries, Equals, int64(2))
	c.Assert(pl.Coordinator.Successes, Equals, int64(2))

	c.Assert(len(pl.Coordinator.SensorReadings), Equals, 20)

	sr := pl.Coordinator.SensorReadings[0]
	c.Assert(sr.SensorID, Equals, "13A20040B421AC")
	c.Assert(sr.BatteryVoltage, Equals, int64(797))
	c.Assert(sr.CPUTemperature, Equals, int64(338))
	c.Assert(sr.SensorTemperature, Equals, int64(621))
	c.Assert(sr.Moisture, Equals, int64(92))
	c.Assert(sr.SendCounter, Equals, int64(18))
	c.Assert(sr.PacketRSSI, Equals, int64(132))
}

func (s *TestSuite) TestPayloadConvertToOldFormat(c *C) {
	b, err := ioutil.ReadFile(filepath.Join("testdata", "example.json"))
	c.Assert(err, IsNil)

	var pl payload
	err = json.Unmarshal(b, &pl)
	c.Assert(err, IsNil)

	c.Assert(pl.Coordinator, Not(IsNil))

	ticks, err := pl.convertToOldFormat()
	c.Assert(err, IsNil)

	c.Assert(ticks, Not(IsNil))
	c.Assert(len(ticks), Equals, 20)

	t := ticks[0]
	c.Assert(t.SensorID, Equals, "13A20040B421AC")
	c.Assert(t.BatteryVoltage, Equals, float64(3.06048))
	c.Assert(t.Temperature, Equals, float64(20.233199999999997))
	c.Assert(t.Humidity, Equals, int64(92))
	c.Assert(t.Sendcounter, Equals, int64(18))
	c.Assert(t.coordinatorID, Equals, fmt.Sprintf("%d", pl.Coordinator.CoordinatorID))
}
