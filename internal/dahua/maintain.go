package dahua

import "fmt"

// AutoReboot is the device's scheduled auto-reboot, read from the AutoMaintain
// config table. Day follows the device's own encoding: on the DH-C5A observed
// live, the AutoMaintain table's factory default sat at Day=0 with a time of
// 02:44, i.e. Day 0 is the "Everyday" slot (0..6 otherwise select a single
// weekday, Sun..Sat). Enable gates whether the schedule runs at all.
type AutoReboot struct {
	Enable bool `json:"enable"`
	Day    int  `json:"day"`
	Hour   int  `json:"hour"`
	Minute int  `json:"minute"`
}

// AutoRebootEveryday is the Day value that reboots the device every day (vs. a
// single weekday). See AutoReboot.Day.
const AutoRebootEveryday = 0

// GetAutoReboot reads the scheduled auto-reboot from the AutoMaintain table.
func (c *Client) GetAutoReboot() (AutoReboot, error) {
	table, err := c.getObjectTable("AutoMaintain")
	if err != nil {
		return AutoReboot{}, err
	}
	ar := AutoReboot{}
	ar.Enable, _ = table["AutoRebootEnable"].(bool)
	ar.Day = toInt(table["AutoRebootDay"])
	ar.Hour = toInt(table["AutoRebootHour"])
	ar.Minute = toInt(table["AutoRebootMinute"])
	return ar, nil
}

// SetAutoReboot writes the scheduled auto-reboot, preserving the rest of the
// AutoMaintain table (the shutdown/startup schedules) via GET-modify-SET. When
// enabling, day/hour/minute are validated; when disabling, only
// AutoRebootEnable is flipped so the previously configured time is retained.
func (c *Client) SetAutoReboot(ar AutoReboot) error {
	if ar.Enable {
		if ar.Day < 0 || ar.Day > 6 {
			return fmt.Errorf("dahua: invalid auto-reboot day %d (0=Everyday/Sun..6=Sat)", ar.Day)
		}
		if ar.Hour < 0 || ar.Hour > 23 {
			return fmt.Errorf("dahua: invalid auto-reboot hour %d", ar.Hour)
		}
		if ar.Minute < 0 || ar.Minute > 59 {
			return fmt.Errorf("dahua: invalid auto-reboot minute %d", ar.Minute)
		}
	}
	table, err := c.getObjectTable("AutoMaintain")
	if err != nil {
		return err
	}
	table["AutoRebootEnable"] = ar.Enable
	if ar.Enable {
		table["AutoRebootDay"] = ar.Day
		table["AutoRebootHour"] = ar.Hour
		table["AutoRebootMinute"] = ar.Minute
	}
	return c.setObjectTable("AutoMaintain", table)
}

// Reboot restarts the device now via the DVRIP RPC magicBox.reboot. It is
// lenient about a dropped connection: the device commonly begins rebooting
// before it flushes the RPC reply, so a transport error after the request was
// sent is treated as "reboot started" rather than a failure.
func (c *Client) Reboot() error {
	resp, err := c.Call("magicBox.reboot", nil)
	if err != nil {
		// Connection reset / timeout right after the call: the device is going
		// down. Best-effort success.
		return nil
	}
	if !resp.ok() {
		return fmt.Errorf("magicBox.reboot failed: %s", respErr(resp))
	}
	return nil
}
