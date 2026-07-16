package dahua

import "fmt"

// GetChannelTitle reads the device's own channel name (ChannelTitle table).
// Confirmed against Dahua's official HTTP API spec
// (docs-sdk/dahua/dahua_http_api_for_ipcsd-v1.40.pdf, section 4.7):
// table.ChannelTitle[Channel].Name.
func (c *Client) GetChannelTitle(ch int) (string, error) {
	table, err := c.getTable("ChannelTitle")
	if err != nil {
		return "", err
	}
	chObj, err := channelObj(table, "ChannelTitle", ch)
	if err != nil {
		return "", err
	}
	name, _ := chObj["Name"].(string)
	return name, nil
}

// SetChannelTitle writes the device's own channel name (ChannelTitle table).
func (c *Client) SetChannelTitle(ch int, name string) error {
	table, err := c.getTable("ChannelTitle")
	if err != nil {
		return err
	}
	chObj, err := channelObj(table, "ChannelTitle", ch)
	if err != nil {
		return err
	}
	chObj["Name"] = name
	return c.setTable("ChannelTitle", table)
}

// ErrOSDUnsupported is returned by GetOSDLines/SetOSDLines when the device's
// VideoWidget[Channel].CustomTitle entries carry no "Text" key. The
// CustomTitle array's position/color/enable fields are confirmed by the
// official spec (section 4.9), but the "Text" field that carries free-text
// content is NOT documented in the locally shipped v1.40 API reference —
// only the wider Dahua firmware/API ecosystem convention (see
// docs/GOTCHAS.md). Live devices whose getConfig response doesn't include
// this key hit this error instead of a silent no-op or corrupted table.
var ErrOSDUnsupported = fmt.Errorf("dahua: CustomTitle[].Text not present in this device's VideoWidget table (unverified field — see docs/GOTCHAS.md)")

// GetOSDLines reads back the free-text custom OSD lines currently configured
// for a channel (VideoWidget[Channel].CustomTitle[index].Text) plus each
// slot's on-screen enable state (.EncodeBlend), in index order (Dahua
// typically exposes up to 4 slots). Returns ErrOSDUnsupported if no slot
// carries a "Text" key.
func (c *Client) GetOSDLines(ch int) (lines []string, enabled []bool, err error) {
	table, err := c.getTable("VideoWidget")
	if err != nil {
		return nil, nil, err
	}
	slots, err := customTitleSlots(table, ch)
	if err != nil {
		return nil, nil, err
	}
	lines = make([]string, len(slots))
	enabled = make([]bool, len(slots))
	found := false
	for i, s := range slots {
		obj, ok := s.(map[string]any)
		if !ok {
			continue
		}
		if t, ok := obj["Text"].(string); ok {
			found = true
			lines[i] = t
		}
		if eb, ok := obj["EncodeBlend"].(bool); ok {
			enabled[i] = eb
		}
	}
	if !found {
		return nil, nil, ErrOSDUnsupported
	}
	return lines, enabled, nil
}

// SetOSDLines writes up to the device's own number of CustomTitle slots
// worth of free-text OSD lines for a channel, plus each slot's on-screen
// enable state (.EncodeBlend). enabled[i] wins when present; a shorter
// enabled slice (or nil, for callers that don't care) falls back to
// enabling exactly the slots getting non-empty text — the old implicit
// behavior — so a slot's visibility isn't silently flipped by a caller that
// only means to change its text. Returns the count actually applied.
// Requires at least one CustomTitle slot to already carry a "Text" key (from
// the GET half of the round-trip) before writing, so an unsupported device
// fails loudly via ErrOSDUnsupported.
func (c *Client) SetOSDLines(ch int, lines []string, enabled []bool) (applied int, err error) {
	table, err := c.getTable("VideoWidget")
	if err != nil {
		return 0, err
	}
	slots, err := customTitleSlots(table, ch)
	if err != nil {
		return 0, err
	}
	hasTextKey := false
	for _, s := range slots {
		if obj, ok := s.(map[string]any); ok {
			if _, ok := obj["Text"]; ok {
				hasTextKey = true
				break
			}
		}
	}
	if !hasTextKey {
		return 0, ErrOSDUnsupported
	}
	n := len(lines)
	if n > len(slots) {
		n = len(slots)
	}
	for i := 0; i < n; i++ {
		obj, ok := slots[i].(map[string]any)
		if !ok {
			continue
		}
		obj["Text"] = lines[i]
		on := lines[i] != ""
		if i < len(enabled) {
			on = enabled[i]
		}
		obj["EncodeBlend"] = on
		applied++
	}
	if err := c.setTable("VideoWidget", table); err != nil {
		return 0, err
	}
	return applied, nil
}

// channelObj navigates table[ch] to its channel object, shared by any table
// shaped as a flat per-channel array (ChannelTitle; VideoWidget uses
// customTitleSlots on top of this for its nested CustomTitle array).
func channelObj(table []any, tableName string, ch int) (map[string]any, error) {
	if ch < 0 || ch >= len(table) {
		return nil, fmt.Errorf("channel %d out of range for %s (have %d)", ch, tableName, len(table))
	}
	chObj, ok := table[ch].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s channel %d: unexpected shape", tableName, ch)
	}
	return chObj, nil
}

// customTitleSlots returns the CustomTitle array for a channel from a
// VideoWidget table (as returned by getTable("VideoWidget")).
func customTitleSlots(table []any, ch int) ([]any, error) {
	chObj, err := channelObj(table, "VideoWidget", ch)
	if err != nil {
		return nil, err
	}
	arr, ok := chObj["CustomTitle"].([]any)
	if !ok {
		return nil, fmt.Errorf("VideoWidget channel %d: no CustomTitle array", ch)
	}
	return arr, nil
}
