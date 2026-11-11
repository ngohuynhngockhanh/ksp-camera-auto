package tvt

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
)

func (c *Client) getBasicCfgXML(ctx context.Context) (DeviceInfo, error) {
	data, err := c.CallXML(ctx, "queryBasicCfg", "")
	if err != nil {
		return DeviceInfo{}, err
	}
	var env struct {
		Status  string `xml:"status"`
		Content struct {
			Name            string `xml:"name"`
			ProductModel    string `xml:"productModel"`
			FirmwareVersion string `xml:"softwareVersion"`
			SerialNumber    string `xml:"sn"`
			DeviceNumber    string `xml:"deviceNumber"`
		} `xml:"content"`
	}
	if err := xml.Unmarshal(data, &env); err != nil {
		return DeviceInfo{}, err
	}
	if !strings.EqualFold(env.Status, "success") {
		return DeviceInfo{}, fmt.Errorf("tvt queryBasicCfg status=%q", env.Status)
	}
	sn := strings.TrimSpace(env.Content.SerialNumber)
	if sn == "" {
		sn = strings.TrimSpace(env.Content.DeviceNumber)
	}
	return DeviceInfo{
		Name:            strings.TrimSpace(env.Content.Name),
		Manufacturer:    "TVT",
		Model:           strings.TrimSpace(env.Content.ProductModel),
		FirmwareVersion: strings.TrimSpace(env.Content.FirmwareVersion),
		SerialNumber:    sn,
	}, nil
}

func parseResString(res string) (int, int) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(res)), "x")
	if len(parts) != 2 {
		return 0, 0
	}
	w, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	return w, h
}

func parseXMLCodec(enct string) (codec string, smart bool) {
	raw := strings.ToUpper(strings.TrimSpace(enct))
	switch raw {
	case "H265S", "H265+", "H.265+", "H265P":
		return "H.265", true
	case "H265", "H.265", "HEVC":
		return "H.265", false
	case "H264S", "H264+", "H.264+", "H264P":
		return "H.264", true
	case "H264", "H.264", "AVC":
		return "H.264", false
	case "MJPEG", "MJPG", "JPEG":
		return "MJPEG", false
	default:
		return raw, false
	}
}

func formatXMLCodec(codec string, smart bool) string {
	base := strings.ToUpper(strings.TrimSpace(codec))
	switch base {
	case "H.265", "H265", "HEVC":
		if smart {
			return "H265S"
		}
		return "H265"
	default:
		if smart {
			return "H264S"
		}
		return "H264"
	}
}

func (c *Client) probeAllXML(ctx context.Context) ([]StreamInfo, error) {
	var out []StreamInfo
	mainXML, err := c.CallXML(ctx, "queryNodeEncodeInfo", `<requireField><name/><chlType/><main/><an/><mn/></requireField>`)
	if err != nil {
		return nil, err
	}
	type streamAttr struct {
		Res     string `xml:"res,attr"`
		FPS     int    `xml:"fps,attr"`
		QoI     int    `xml:"QoI,attr"`
		Audio   string `xml:"audio,attr"`
		BitType string `xml:"bitType,attr"`
		Enct    string `xml:"enct,attr"`
		GOP     int    `xml:"GOP,attr"`
		AGOP    int    `xml:"aGOP,attr"`
		MGOP    int    `xml:"mGOP,attr"`
	}
	var mainEnv struct {
		Status  string `xml:"status"`
		Content struct {
			Items []struct {
				ID   int        `xml:"id,attr"`
				Name string     `xml:"name"`
				Main streamAttr `xml:"main"`
				AN   streamAttr `xml:"an"`
				MN   streamAttr `xml:"mn"`
			} `xml:"item"`
		} `xml:"content"`
	}
	if err := xml.Unmarshal(mainXML, &mainEnv); err != nil {
		return nil, err
	}
	for _, it := range mainEnv.Content.Items {
		st := it.AN
		if st.Res == "" {
			st = it.MN
		}
		w, h := parseResString(st.Res)
		if w <= 0 || it.ID <= 0 {
			continue
		}
		codec, smart := parseXMLCodec(it.Main.Enct)
		gop := it.Main.AGOP
		if gop <= 0 {
			gop = it.Main.MGOP
		}
		if gop <= 0 {
			gop = st.FPS * 2
		}
		mode := strings.ToUpper(strings.TrimSpace(st.BitType))
		if mode == "" {
			mode = "VBR"
		}
		out = append(out, StreamInfo{
			Channel:        it.ID,
			Stream:         0,
			Width:          w,
			Height:         h,
			FPS:            st.FPS,
			Compression:    codec,
			SmartCodec:     smart,
			AudioCodec:     "G.711A",
			AudioEnable:    strings.EqualFold(st.Audio, "true") || st.Audio == "1",
			GOP:            gop,
			BitRate:        st.QoI,
			BitRateControl: mode,
			Name:           strings.TrimSpace(it.Name),
		})
	}

	if subXML, err := c.CallXML(ctx, "queryNetworkNodeEncodeInfo", `<requireField><name/><sub/></requireField>`); err == nil {
		var subEnv struct {
			Content struct {
				Items []struct {
					ID   int        `xml:"id,attr"`
					Name string     `xml:"name"`
					Sub  streamAttr `xml:"sub"`
				} `xml:"item"`
			} `xml:"content"`
		}
		if xml.Unmarshal(subXML, &subEnv) == nil {
			for _, it := range subEnv.Content.Items {
				w, h := parseResString(it.Sub.Res)
				if w <= 0 || it.ID <= 0 {
					continue
				}
				codec, smart := parseXMLCodec(it.Sub.Enct)
				mode := strings.ToUpper(strings.TrimSpace(it.Sub.BitType))
				if mode == "" {
					mode = "CBR"
				}
				out = append(out, StreamInfo{
					Channel:        it.ID,
					Stream:         1,
					Width:          w,
					Height:         h,
					FPS:            it.Sub.FPS,
					Compression:    codec,
					SmartCodec:     smart,
					GOP:            it.Sub.GOP,
					BitRate:        it.Sub.QoI,
					BitRateControl: mode,
					Name:           strings.TrimSpace(it.Name),
				})
			}
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Channel != out[j].Channel {
			return out[i].Channel < out[j].Channel
		}
		return out[i].Stream < out[j].Stream
	})
	return out, nil
}

func (c *Client) getStreamInfoXML(ctx context.Context, channel0Based, stream int) (StreamInfo, error) {
	all, err := c.probeAllXML(ctx)
	if err != nil {
		return StreamInfo{}, err
	}
	ch1 := channel0Based + 1
	for _, s := range all {
		if s.Channel == ch1 && s.Stream == stream {
			return s, nil
		}
	}
	return StreamInfo{}, fmt.Errorf("tvt xml stream not found ch=%d stream=%d", ch1, stream)
}

// SetStreamConfig updates codec, resolution, framerate, GOP, bitrate, or smart codec via NVMS-9000 XML.
func (c *Client) SetStreamConfig(ctx context.Context, channel0Based, stream int, mutate func(*StreamInfo)) error {
	cur, err := c.GetStreamInfo(ctx, channel0Based, stream)
	if err != nil {
		return err
	}
	mutate(&cur)
	ch1 := channel0Based + 1
	enct := formatXMLCodec(cur.Compression, cur.SmartCodec)
	resStr := fmt.Sprintf("%dx%d", cur.Width, cur.Height)
	mode := cur.BitRateControl
	if mode == "" {
		mode = "VBR"
	}
	gop := cur.GOP
	if gop <= 0 {
		gop = cur.FPS * 2
	}
	if stream > 0 {
		inner := fmt.Sprintf(`<content type="list" total="1"><item id="%d"><sub res="%s" fps="%d" QoI="%d" bitType="%s" enct="%s" GOP="%d"></sub></item></content>`,
			ch1, resStr, cur.FPS, cur.BitRate, mode, enct, gop)
		resp, err := c.CallXML(ctx, "editNetworkNodeEncodeInfo", inner)
		if err != nil {
			return err
		}
		return checkXMLSuccess("editNetworkNodeEncodeInfo", resp)
	}
	aud := "false"
	if cur.AudioEnable {
		aud = "true"
	}
	inner := fmt.Sprintf(`<content type="list" total="1"><item id="%d"><an res="%s" fps="%d" QoI="%d" audio="%s" bitType="%s"></an><main enct="%s" aGOP="%d"></main></item></content>`,
		ch1, resStr, cur.FPS, cur.BitRate, aud, mode, enct, gop)
	resp, err := c.CallXML(ctx, "editNodeEncodeInfo", inner)
	if err != nil {
		return err
	}
	return checkXMLSuccess("editNodeEncodeInfo", resp)
}

func (c *Client) getDevListXML(ctx context.Context) ([]dahua.RemoteChannel, error) {
	data, err := c.CallXML(ctx, "queryDevList", `<requireField><name/><ip/><port/><userName/><protocolType/><productModel/><chlIndex/><index/></requireField>`)
	if err != nil {
		return nil, err
	}
	var env struct {
		Status  string `xml:"status"`
		Content struct {
			Items []struct {
				ID           int    `xml:"id,attr"`
				Name         string `xml:"name"`
				IP           string `xml:"ip"`
				Port         int    `xml:"port"`
				UserName     string `xml:"userName"`
				ChlIndex     int    `xml:"chlIndex"`
				ProductModel string `xml:"productModel"`
			} `xml:"item"`
		} `xml:"content"`
	}
	if err := xml.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	if !strings.EqualFold(env.Status, "success") {
		return nil, fmt.Errorf("tvt queryDevList status=%q", env.Status)
	}
	var out []dahua.RemoteChannel
	for _, it := range env.Content.Items {
		ch1 := it.ID
		if it.ChlIndex > 0 {
			ch1 = it.ChlIndex
		}
		if ch1 <= 0 {
			continue
		}
		out = append(out, dahua.RemoteChannel{
			Channel: ch1 - 1,
			Address: strings.TrimSpace(it.IP),
			Port:    it.Port,
			Name:    strings.TrimSpace(it.Name),
			Enable:  true,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Channel < out[j].Channel })
	return out, nil
}

// GetChannelOSD reads channel title and watermark text for 0-based channel.
func (c *Client) GetChannelOSD(ctx context.Context, channel0Based int) (name string, lines []string, enabled []bool, supported bool, err error) {
	ch1 := channel0Based + 1
	if err := c.ensureAuth(ctx); err != nil {
		return "", nil, nil, false, err
	}
	c.mu.Lock()
	xmlOK := c.xmlAuthed
	c.mu.Unlock()
	if !xmlOK {
		return fmt.Sprintf("Channel %02d", ch1), nil, nil, false, nil
	}
	data, err := c.CallXML(ctx, "queryIPChlORChlOSD", fmt.Sprintf(`<condition><chlId>%d</chlId></condition>`, ch1))
	if err != nil {
		return "", nil, nil, false, err
	}
	var env struct {
		Status  string `xml:"status"`
		Content struct {
			Chl struct {
				ChlName struct {
					Switch string `xml:"switch"`
					Name   string `xml:"name"`
				} `xml:"chlName"`
			} `xml:"chl"`
		} `xml:"content"`
	}
	if err := xml.Unmarshal(data, &env); err != nil {
		return "", nil, nil, false, err
	}
	name = strings.TrimSpace(env.Content.Chl.ChlName.Name)
	show := strings.EqualFold(env.Content.Chl.ChlName.Switch, "true")
	return name, []string{name}, []bool{show}, true, nil
}

// SetChannelName updates a channel's display name via NVMS-9000 /editDev.
func (c *Client) SetChannelName(ctx context.Context, channel0Based int, name string) error {
	ch1 := channel0Based + 1
	resp, err := c.CallXML(ctx, "editDev", fmt.Sprintf(`<content><id>%d</id><name><![CDATA[%s]]></name></content>`, ch1, name))
	if err != nil {
		return err
	}
	return checkXMLSuccess("editDev", resp)
}

// SetOSD updates channel watermark / OSD overlay lines via NVMS-9000 XML.
func (c *Client) SetOSD(ctx context.Context, channel0Based int, lines []string, enabled []bool) (int, error) {
	ch1 := channel0Based + 1
	text := ""
	if len(lines) > 0 {
		text = strings.TrimSpace(lines[0])
	}
	sw := "true"
	if len(enabled) > 0 && !enabled[0] {
		sw = "false"
	}
	resp, err := c.CallXML(ctx, "editChlWaterMark", fmt.Sprintf(`<content><chl id="%d"><watermark><value>%s</value><switch>%s</switch></watermark></chl></content>`,
		ch1, xmlEscapeText(text), sw))
	if err != nil {
		return 0, err
	}
	if err := checkXMLSuccess("editChlWaterMark", resp); err != nil {
		return 0, err
	}
	return 1, nil
}

func (c *Client) snapshotXML(ctx context.Context, channel0Based int) ([]byte, error) {
	ch1 := channel0Based + 1
	data, err := c.CallXML(ctx, "snapChlPicture", fmt.Sprintf(`<condition><chlId>%d</chlId></condition>`, ch1))
	if err != nil {
		return nil, err
	}
	var env struct {
		Status  string `xml:"status"`
		Content struct {
			Item string `xml:"item"`
		} `xml:"content"`
	}
	if err := xml.Unmarshal(data, &env); err != nil || !strings.EqualFold(env.Status, "success") {
		return nil, fmt.Errorf("tvt snapChlPicture failed")
	}
	raw := strings.TrimSpace(env.Content.Item)
	if idx := strings.Index(raw, ","); idx >= 0 {
		raw = raw[idx+1:]
	}
	if raw == "" {
		return nil, fmt.Errorf("tvt snapChlPicture empty")
	}
	return base64.StdEncoding.DecodeString(raw)
}

func (c *Client) findRecordingsXML(ctx context.Context, channel0Based int, start, end time.Time) ([]dahua.Recording, error) {
	ch1 := channel0Based + 1
	inner := fmt.Sprintf(`<requireField><chl/><recList><item><recType/><startTime/><endTime/><size/></item></recList></requireField><condition><modeType>TIME</modeType><startTime>%s</startTime><endTime>%s</endTime><startTimeEx>%s</startTimeEx><endTimeEx>%s</endTimeEx><chl id="%d"></chl></condition>`,
		start.In(time.Local).Format("2006-01-02 15:04:05"),
		end.In(time.Local).Format("2006-01-02 15:04:05"),
		start.UTC().Format("2006-01-02 15:04:05"),
		end.UTC().Format("2006-01-02 15:04:05"),
		ch1,
	)
	data, err := c.CallXML(ctx, "queryChlRecLog", inner)
	if err != nil {
		return nil, err
	}
	var env struct {
		Status  string `xml:"status"`
		Content struct {
			RecList struct {
				Items []struct {
					RecType   string `xml:"recType"`
					StartTime string `xml:"startTime"`
					EndTime   string `xml:"endTime"`
					Size      int64  `xml:"size"`
				} `xml:"item"`
			} `xml:"recList"`
		} `xml:"content"`
	}
	if err := xml.Unmarshal(data, &env); err != nil || !strings.EqualFold(env.Status, "success") {
		return nil, fmt.Errorf("tvt queryChlRecLog failed")
	}
	var out []dahua.Recording
	for _, it := range env.Content.RecList.Items {
		st, err1 := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(it.StartTime), time.UTC)
		et, err2 := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(it.EndTime), time.UTC)
		if err1 != nil || err2 != nil || !et.After(st) {
			continue
		}
		out = append(out, makeDahuaRecording(channel0Based, st, et, it.Size*1024*1024))
	}
	return coalesceRecordings(out), nil
}

// GetStorageInfo queries NVR HDD capacity via NVMS-9000 /queryDiskInfo.
func (c *Client) GetStorageInfo(ctx context.Context) ([]dahua.StorageDevice, error) {
	data, err := c.CallXML(ctx, "queryDiskInfo", "")
	if err != nil {
		return nil, err
	}
	var env struct {
		Status  string `xml:"status"`
		Content struct {
			Items []struct {
				ID         string  `xml:"id,attr"`
				Name       string  `xml:"name"`
				TotalSpace float64 `xml:"size"`
				FreeSpace  float64 `xml:"freeSpace"`
				DiskStatus string  `xml:"diskStatus"`
			} `xml:"item"`
		} `xml:"content"`
	}
	if err := xml.Unmarshal(data, &env); err != nil || !strings.EqualFold(env.Status, "success") {
		return nil, fmt.Errorf("tvt queryDiskInfo failed")
	}
	var out []dahua.StorageDevice
	for i, d := range env.Content.Items {
		nm := strings.TrimSpace(d.Name)
		if nm == "" {
			nm = fmt.Sprintf("HDD-%d", i+1)
		}
		// NVMS-9000 reports disk sizes in MB -> convert to bytes.
		tot := int64(d.TotalSpace * 1024 * 1024)
		free := int64(d.FreeSpace * 1024 * 1024)
		used := tot - free
		if used < 0 {
			used = 0
		}
		out = append(out, dahua.StorageDevice{
			Name:  nm,
			State: strings.TrimSpace(d.DiskStatus),
			Details: []dahua.StorageDetail{{
				Path:       nm,
				Type:       "ReadWrite",
				TotalBytes: tot,
				UsedBytes:  used,
			}},
		})
	}
	return out, nil
}

func (c *Client) rebootXML(ctx context.Context) error {
	resp, err := c.CallXML(ctx, "reboot", "")
	if err != nil {
		return err
	}
	return checkXMLSuccess("reboot", resp)
}

func checkXMLSuccess(action string, raw []byte) error {
	var env struct {
		Status    string `xml:"status"`
		ErrorCode string `xml:"errorCode"`
	}
	if err := xml.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("tvt %s decode: %w", action, err)
	}
	if !strings.EqualFold(env.Status, "success") {
		return fmt.Errorf("tvt %s failed (errorCode=%s)", action, strings.TrimSpace(env.ErrorCode))
	}
	return nil
}
