package dahua

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// RemoteChannel is one NVR channel's connected (remote) camera, from the NVR's
// RemoteDevice config. Channel is 0-based (the NVR's channel index).
type RemoteChannel struct {
	Channel int    `json:"channel"`
	Address string `json:"address"` // the connected camera's IP
	Port    int    `json:"port"`
	Name    string `json:"name"`
	Enable  bool   `json:"enable"`
}

// GetRemoteDevices reads a Dahua NVR's RemoteDevice config — the map of channel
// → connected camera. Unlike most configs its `table` is a JSON OBJECT keyed by
// "uuid:System_CONFIG_NETCAMERA_INFO_<N>" (N = 0-based channel), not an array,
// so it can't use getTable; it parses the map and pulls N from the key suffix.
// Verified live against an NVR: each value carries Address (camera IP), Name,
// Enable, Port. Returned sorted by channel.
func (c *Client) GetRemoteDevices() ([]RemoteChannel, error) {
	resp, err := c.Call("configManager.getConfig", map[string]any{"name": "RemoteDevice"})
	if err != nil {
		return nil, err
	}
	if !resp.ok() {
		return nil, fmt.Errorf("getConfig RemoteDevice failed: %s", respErr(resp))
	}
	var p struct {
		Table map[string]struct {
			Address string `json:"Address"`
			Port    int    `json:"Port"`
			Name    string `json:"Name"`
			Enable  bool   `json:"Enable"`
		} `json:"table"`
	}
	if err := json.Unmarshal(resp.Params, &p); err != nil {
		return nil, fmt.Errorf("getConfig RemoteDevice: decode: %w", err)
	}
	out := make([]RemoteChannel, 0, len(p.Table))
	for key, v := range p.Table {
		ch := channelFromRemoteKey(key)
		if ch < 0 {
			continue
		}
		out = append(out, RemoteChannel{Channel: ch, Address: v.Address, Port: v.Port, Name: v.Name, Enable: v.Enable})
	}
	// insertion order from a map is random; sort by channel
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].Channel > out[j].Channel; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out, nil
}

// channelFromRemoteKey extracts the 0-based channel index from a RemoteDevice
// table key like "uuid:System_CONFIG_NETCAMERA_INFO_7" -> 7. Returns -1 if the
// trailing segment isn't an integer.
func channelFromRemoteKey(key string) int {
	i := strings.LastIndex(key, "_")
	if i < 0 || i+1 >= len(key) {
		return -1
	}
	n, err := strconv.Atoi(key[i+1:])
	if err != nil {
		return -1
	}
	return n
}
