package dahua

import (
	"encoding/json"
	"fmt"
	"time"
)

// deviceTimeLayout is Dahua's device-local timestamp format used by
// mediaFileFind conditions and file infos ("2026-07-18 21:31:07").
const deviceTimeLayout = "2006-01-02 15:04:05"

// Recording is one recorded file segment on the device, as returned by
// mediaFileFind.findNextFile. Times are the device's own local times (no
// timezone). FilePath is the on-device path (e.g. under /mnt/sd) — kept for
// reference/debugging; playback is by time range over RTSP, not by path.
type Recording struct {
	Channel     int      `json:"channel"`
	StartTime   string   `json:"startTime"`
	EndTime     string   `json:"endTime"`
	Duration    int      `json:"duration"` // seconds
	Type        string   `json:"type"`     // "dav"
	VideoStream string   `json:"videoStream"`
	Length      int64    `json:"length"` // bytes
	FilePath    string   `json:"filePath"`
	Events      []string `json:"events"`
}

// FindRecordings lists recorded files on a channel between start and end via
// the mediaFileFind interface: create a finder object, run findFile with the
// time-range condition, then page through findNextFile until the device
// reports fewer than the requested count. The finder is always closed and
// destroyed. channel is 0-based (matching Profile.Channel and the device's own
// Channel field). It rides the existing DVRIP session — no HTTP/CGI needed.
func (c *Client) FindRecordings(channel int, start, end time.Time) ([]Recording, error) {
	cr, err := c.Call("mediaFileFind.factory.create", nil)
	if err != nil {
		return nil, err
	}
	var objID int64
	_ = json.Unmarshal(cr.Result, &objID)
	if objID == 0 {
		return nil, fmt.Errorf("dahua: mediaFileFind.factory.create returned no object (result=%s)", string(cr.Result))
	}
	defer func() {
		_, _ = c.CallObject("mediaFileFind.close", objID, nil)
		_, _ = c.CallObject("mediaFileFind.destroy", objID, nil)
	}()

	fr, err := c.CallObject("mediaFileFind.findFile", objID, map[string]any{
		"condition": map[string]any{
			"Channel":   channel,
			"Types":     []string{"dav"},
			"Flags":     []string{"Timing", "Event", "Manual"},
			"StartTime": start.Format(deviceTimeLayout),
			"EndTime":   end.Format(deviceTimeLayout),
		},
	})
	if err != nil {
		return nil, err
	}
	if !fr.ok() {
		// A range with no recordings returns a non-OK "no data" — surface it as
		// an empty list rather than an error so the timeline just shows nothing.
		return nil, nil
	}

	const pageSize = 100
	var out []Recording
	for {
		nr, err := c.CallObject("mediaFileFind.findNextFile", objID, map[string]any{"count": pageSize})
		if err != nil {
			return out, nil // return what we have; a mid-page error ends paging
		}
		var p struct {
			Found int `json:"found"`
			Infos []struct {
				Channel     int      `json:"Channel"`
				StartTime   string   `json:"StartTime"`
				EndTime     string   `json:"EndTime"`
				Duration    int      `json:"Duration"`
				Type        string   `json:"Type"`
				VideoStream string   `json:"VideoStream"`
				Length      int64    `json:"Length"`
				FilePath    string   `json:"FilePath"`
				Events      []string `json:"Events"`
			} `json:"infos"`
		}
		if err := json.Unmarshal(nr.Params, &p); err != nil {
			return out, nil
		}
		for _, in := range p.Infos {
			out = append(out, Recording{
				Channel:     in.Channel,
				StartTime:   in.StartTime,
				EndTime:     in.EndTime,
				Duration:    in.Duration,
				Type:        in.Type,
				VideoStream: in.VideoStream,
				Length:      in.Length,
				FilePath:    in.FilePath,
				Events:      in.Events,
			})
		}
		if p.Found < pageSize {
			break
		}
		// Safety bound: a full day of dense event recordings is a few hundred
		// segments; stop far above that rather than loop forever on a device
		// that never reports a short page.
		if len(out) >= 20000 {
			break
		}
	}
	return out, nil
}
