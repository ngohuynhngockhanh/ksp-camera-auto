package isapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Video codec values accepted by StreamingChannel.Video.VideoCodecType.
const (
	CodecH264  = "H.264"
	CodecH265  = "H.265"
	CodecMJPEG = "MJPEG"
)

// Transport carries one ISAPI request (an HTTP-style method + "/ISAPI/..."
// path, optional XML body) to a device and returns the response body. The
// default implementation is HTTP+Digest; the optional cgo `hiksdk` backend
// implements it over NET_DVR_STDXMLConfig so the exact same XML get/set logic
// drives a device reachable only on the private port 8000.
type Transport interface {
	Do(ctx context.Context, method, path string, body []byte) ([]byte, error)
}

// Client talks ISAPI (Hikvision's HTTP/XML control API) to one device over a
// pluggable Transport.
type Client struct {
	rt Transport
}

// New builds a Client for the device at host:port over HTTP(S). https selects
// the scheme; when true, TLS certificate verification is skipped because
// Hikvision devices ship self-signed certificates by default. timeout bounds
// every request (connect + read).
func New(host string, port int, https bool, user, pass string, timeout time.Duration) *Client {
	scheme := "http"
	var tlsConf *tls.Config
	if https {
		scheme = "https"
		tlsConf = &tls.Config{InsecureSkipVerify: true} // #nosec G402 -- Hikvision devices use self-signed certs by default
	}
	baseTransport := &http.Transport{TLSClientConfig: tlsConf}
	digest := NewDigestTransport(user, pass, baseTransport)
	return &Client{rt: &httpTransport{
		baseURL: fmt.Sprintf("%s://%s:%d", scheme, host, port),
		http:    &http.Client{Transport: digest, Timeout: timeout},
		user:    user,
		pass:    pass,
		base:    baseTransport,
	}}
}

// NewWithTransport builds a Client over a custom Transport (e.g. the SDK
// backend). All GET/PUT/Set logic is shared with the HTTP client.
func NewWithTransport(rt Transport) *Client { return &Client{rt: rt} }

// StreamingChannel mirrors the subset of ISAPI's
// /ISAPI/Streaming/channels/{id} document this package understands. GET
// unmarshals a device's response into this struct; PUT marshals it back.
//
// NOTE: only the fields listed below round-trip. A real StreamingChannel
// document also carries Transport/Unicast/RTSP fields this milestone doesn't
// touch; PutStreamChannel does not preserve them because it always starts
// from a struct populated by a prior GetStreamChannel call in the same
// process (GET-modify-PUT), so any field this struct doesn't model is lost
// on PUT. That's acceptable for the payload/transport layer this milestone
// delivers; a later milestone can widen the struct if a real device rejects
// the trimmed document.
type StreamingChannel struct {
	XMLName     xml.Name `xml:"StreamingChannel"`
	Xmlns       string   `xml:"xmlns,attr,omitempty"`
	ID          string   `xml:"id"`
	ChannelName string   `xml:"channelName,omitempty"`
	Enabled     bool     `xml:"enabled"`
	Video       *Video   `xml:"Video"`
	Audio       *Audio   `xml:"Audio,omitempty"`
}

// Video is StreamingChannel.Video.
type Video struct {
	Enabled                 bool        `xml:"enabled"`
	VideoCodecType          string      `xml:"videoCodecType,omitempty"`
	VideoResolutionWidth    int         `xml:"videoResolutionWidth,omitempty"`
	VideoResolutionHeight   int         `xml:"videoResolutionHeight,omitempty"`
	MaxFrameRate            int         `xml:"maxFrameRate,omitempty"` // fps*100, e.g. 2500 = 25fps
	VideoQualityControlType string      `xml:"videoQualityControlType,omitempty"`
	GovLength               int         `xml:"GovLength,omitempty"`
	ConstantBitRate         int         `xml:"constantBitRate,omitempty"`
	VBRUpperCap             int         `xml:"vbrUpperCap,omitempty"`
	VBRAverageCap           int         `xml:"vbrAverageCap,omitempty"`
	SmartCodec              *SmartCodec `xml:"SmartCodec,omitempty"`
}

// SmartCodec toggles Hikvision's H.264+/H.265+ compression, either inline
// under StreamingChannel.Video.SmartCodec or as the standalone sub-resource
// /ISAPI/Streaming/channels/{id}/smartCodec.
type SmartCodec struct {
	Enabled bool `xml:"enabled"`
}

// Audio is StreamingChannel.Audio.
type Audio struct {
	Enabled              bool   `xml:"enabled"`
	AudioCompressionType string `xml:"audioCompressionType,omitempty"`
}

// responseStatus is the standard ISAPI success/failure envelope returned by
// state-changing PUT/POST requests.
type responseStatus struct {
	XMLName       xml.Name `xml:"ResponseStatus"`
	StatusCode    int      `xml:"statusCode"`
	StatusString  string   `xml:"statusString"`
	SubStatusCode string   `xml:"subStatusCode"`
}

// checkResponseStatus parses body as a ResponseStatus envelope and returns an
// error unless statusCode == 1 ("OK").
func checkResponseStatus(body []byte) error {
	var rs responseStatus
	if err := xml.Unmarshal(body, &rs); err != nil {
		return fmt.Errorf("isapi: decode ResponseStatus: %w (body: %s)", err, truncate(body, 200))
	}
	if rs.StatusCode != 1 {
		return fmt.Errorf("isapi: statusCode=%d statusString=%q subStatusCode=%q", rs.StatusCode, rs.StatusString, rs.SubStatusCode)
	}
	return nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

// do routes one request through the client's Transport.
func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	return c.rt.Do(ctx, method, path, body)
}

// httpTransport is the default Transport: ISAPI over HTTP(S) with Digest auth.
// user/pass/base are kept alongside the (timeout-bound) http client so
// SearchTrack/DownloadStream can build a fresh, timeout-UNBOUND client for
// requests whose responses are too large or too slow for the config-call
// path — see streamClient.
type httpTransport struct {
	baseURL    string
	http       *http.Client
	user, pass string
	base       http.RoundTripper
}

// Do issues an HTTP request against path (which must start with "/ISAPI"),
// authenticating via the DigestTransport, and returns the response body.
// Non-2xx HTTP statuses are still returned as data (some ISAPI errors carry a
// useful ResponseStatus body alongside a 4xx) but also as an error.
func (c *httpTransport) Do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	resp, err := c.request(ctx, c.http, method, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("isapi: read response %s %s: %w", method, path, err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return data, fmt.Errorf("isapi: %s %s: unauthorized (digest auth failed): %s", method, path, truncate(data, 300))
	}
	if resp.StatusCode >= 300 {
		if statusErr := checkResponseStatus(data); statusErr != nil {
			return data, fmt.Errorf("isapi: %s %s: HTTP %d: %w", method, path, resp.StatusCode, statusErr)
		}
		return data, fmt.Errorf("isapi: %s %s: HTTP %d: %s", method, path, resp.StatusCode, truncate(data, 300))
	}
	return data, nil
}

// request builds and sends one HTTP request against path through the given
// client, leaving the response for the caller to read/close — factored out of
// Do so rawRequest (SearchTrack/DownloadStream's unbounded path) can share the
// exact same request-building logic against a different client.
func (c *httpTransport) request(ctx context.Context, client *http.Client, method, path string, body []byte) (*http.Response, error) {
	url := c.baseURL + path
	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("isapi: build request %s %s: %w", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/xml")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("isapi: %s %s: %w", method, path, err)
	}
	return resp, nil
}

// streamClient returns a FRESH HTTP+Digest client with no read timeout, for
// SearchTrack/DownloadStream: a dense recording search can return more than
// the 1 MiB Do caps at, and a native-container download can be gigabytes and
// take far longer than the per-request timeout config calls use (that timeout
// is enforced by http.Client.Timeout, which — unlike a dial/header timeout —
// also bounds the time spent reading the response body, so reusing c.http
// would truncate a long download). A fresh DigestTransport costs one extra
// 401 round trip instead of reusing New's cached challenge, an acceptable
// rare-call price.
func (c *httpTransport) streamClient() *http.Client {
	return &http.Client{Transport: NewDigestTransport(c.user, c.pass, c.base)}
}

// rawRequest is like request but always goes over streamClient's unbound
// client — the shared entry point for SearchTrack (doUnbounded) and
// DownloadStream.
func (c *httpTransport) rawRequest(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	return c.request(ctx, c.streamClient(), method, path, body)
}

// channelID computes the compound ISAPI streaming-channel id from a native
// (1-based) Hikvision channel number and a 0-based stream index (0=main,
// 1=sub1, 2=sub2 — matching camera.Stream). channelID(1, 0) == 101.
func channelID(ch, stream int) int {
	return ch*100 + stream + 1
}

// GetStreamChannel fetches and parses /ISAPI/Streaming/channels/{id}. id is
// the compound channel id (e.g. 101), typically produced by channelID.
func (c *Client) GetStreamChannel(ctx context.Context, id int) (*StreamingChannel, error) {
	path := fmt.Sprintf("/ISAPI/Streaming/channels/%d", id)
	body, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var sc StreamingChannel
	if err := xml.Unmarshal(body, &sc); err != nil {
		return nil, fmt.Errorf("isapi: decode StreamingChannel %d: %w (body: %s)", id, err, truncate(body, 200))
	}
	return &sc, nil
}

// PutStreamChannel writes sc back to /ISAPI/Streaming/channels/{id} and
// verifies the device accepted it (ResponseStatus.statusCode == 1).
func (c *Client) PutStreamChannel(ctx context.Context, id int, sc *StreamingChannel) error {
	if sc.Xmlns == "" {
		sc.Xmlns = "http://www.hikvision.com/ver20/XMLSchema"
	}
	body, err := xml.Marshal(sc)
	if err != nil {
		return fmt.Errorf("isapi: encode StreamingChannel %d: %w", id, err)
	}
	full := append([]byte(xml.Header), body...)
	path := fmt.Sprintf("/ISAPI/Streaming/channels/%d", id)
	respBody, err := c.do(ctx, http.MethodPut, path, full)
	if err != nil {
		return err
	}
	if err := checkResponseStatus(respBody); err != nil {
		return fmt.Errorf("isapi: PUT %s: %w", path, err)
	}
	return nil
}

// getSmartCodec fetches the standalone smartCodec sub-resource for a
// compound channel id.
func (c *Client) getSmartCodec(ctx context.Context, id int) (SmartCodec, error) {
	path := fmt.Sprintf("/ISAPI/Streaming/channels/%d/smartCodec", id)
	body, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return SmartCodec{}, err
	}
	var sc SmartCodec
	if err := xml.Unmarshal(body, &sc); err != nil {
		return SmartCodec{}, fmt.Errorf("isapi: decode smartCodec %d: %w (body: %s)", id, err, truncate(body, 200))
	}
	return sc, nil
}

// putSmartCodec writes the standalone smartCodec sub-resource for a compound
// channel id.
func (c *Client) putSmartCodec(ctx context.Context, id int, on bool) error {
	body, err := xml.Marshal(SmartCodec{Enabled: on})
	if err != nil {
		return fmt.Errorf("isapi: encode smartCodec %d: %w", id, err)
	}
	full := append([]byte(xml.Header), body...)
	path := fmt.Sprintf("/ISAPI/Streaming/channels/%d/smartCodec", id)
	respBody, err := c.do(ctx, http.MethodPut, path, full)
	if err != nil {
		return err
	}
	if err := checkResponseStatus(respBody); err != nil {
		return fmt.Errorf("isapi: PUT %s: %w", path, err)
	}
	return nil
}

// streamPath is the ISAPI resource for a compound channel id.
func streamPath(id int) string {
	return fmt.Sprintf("/ISAPI/Streaming/channels/%d", id)
}

// mutateStreamChannel does a GET-modify-PUT that preserves the FULL device
// document, replacing only the given <tag>value</tag> pairs in the raw XML.
// Re-marshalling a trimmed Go struct is rejected by real devices/NVRs with
// "Invalid XML Content" because the schema requires fields this package does
// not model, so we edit the raw bytes instead.
func (c *Client) mutateStreamChannel(ctx context.Context, id int, edits map[string]string) error {
	path := streamPath(id)
	raw, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	for tag, val := range edits {
		raw = replaceXMLTag(raw, tag, val)
	}
	resp, err := c.do(ctx, http.MethodPut, path, raw)
	if err != nil {
		return err
	}
	if err := checkResponseStatus(resp); err != nil {
		return fmt.Errorf("isapi: PUT %s: %w", path, err)
	}
	return nil
}

// replaceXMLTag replaces the content of the first <tag>...</tag> in doc with
// value. If the tag is absent the document is returned unchanged.
func replaceXMLTag(doc []byte, tag, value string) []byte {
	open := []byte("<" + tag + ">")
	closeTag := []byte("</" + tag + ">")
	i := bytes.Index(doc, open)
	if i < 0 {
		return doc
	}
	start := i + len(open)
	rel := bytes.Index(doc[start:], closeTag)
	if rel < 0 {
		return doc
	}
	end := start + rel
	out := make([]byte, 0, len(doc)-(end-start)+len(value))
	out = append(out, doc[:start]...)
	out = append(out, value...)
	out = append(out, doc[end:]...)
	return out
}

// hasXMLTag reports whether doc contains an opening <tag> element.
func hasXMLTag(doc []byte, tag string) bool { return bytes.Contains(doc, []byte("<"+tag+">")) }

// extractXMLString returns the content of the first <tag>...</tag> in doc,
// or "" if the tag is absent.
func extractXMLString(doc []byte, tag string) string {
	open := []byte("<" + tag + ">")
	closeTag := []byte("</" + tag + ">")
	i := bytes.Index(doc, open)
	if i < 0 {
		return ""
	}
	start := i + len(open)
	rel := bytes.Index(doc[start:], closeTag)
	if rel < 0 {
		return ""
	}
	return string(doc[start : start+rel])
}

// extractXMLInt returns the integer content of the first <tag>...</tag> in
// doc, or 0 if the tag is absent or its content isn't a valid integer.
func extractXMLInt(doc []byte, tag string) int {
	s := extractXMLString(doc, tag)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// extractXMLInBlock returns the content of the first <tag>...</tag> that
// occurs INSIDE the first <block>...</block> in doc, or "" if either is
// absent. Mirrors replaceXMLTagInBlock's scoping.
func extractXMLInBlock(doc []byte, block, tag string) string {
	open := []byte("<" + block + ">")
	closeB := []byte("</" + block + ">")
	i := bytes.Index(doc, open)
	if i < 0 {
		return ""
	}
	rel := bytes.Index(doc[i:], closeB)
	if rel < 0 {
		return ""
	}
	end := i + rel
	return extractXMLString(doc[i:end], tag)
}

// mutateStreamChannelStrict is like mutateStreamChannel but fails if any edit
// tag is absent from the device document, so a setter cannot silently no-op
// (replaceXMLTag leaves an absent tag unchanged).
func (c *Client) mutateStreamChannelStrict(ctx context.Context, id int, edits map[string]string) error {
	path := streamPath(id)
	raw, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	for tag := range edits {
		if !hasXMLTag(raw, tag) {
			return fmt.Errorf("isapi: channel %d: <%s> not in StreamingChannel document (firmware does not expose it)", id, tag)
		}
	}
	for tag, val := range edits {
		raw = replaceXMLTag(raw, tag, val)
	}
	resp, err := c.do(ctx, http.MethodPut, path, raw)
	if err != nil {
		return err
	}
	if err := checkResponseStatus(resp); err != nil {
		return fmt.Errorf("isapi: PUT %s: %w", path, err)
	}
	return nil
}

// gopEdits returns the raw-XML edits to set the I-frame interval to gopFrames.
// Prefers <GovLength> (frames). If only <keyFrameInterval> exists it is used;
// when kfiIsMS the value is converted frames->ms via fps from <maxFrameRate>.
func gopEdits(raw []byte, gopFrames int, kfiIsMS bool) (map[string]string, error) {
	if hasXMLTag(raw, "GovLength") {
		return map[string]string{"GovLength": strconv.Itoa(gopFrames)}, nil
	}
	if hasXMLTag(raw, "keyFrameInterval") {
		if kfiIsMS {
			fps := 0
			if m := extractXMLInt(raw, "maxFrameRate"); m > 0 {
				fps = m / 100
			}
			if fps <= 0 {
				return nil, fmt.Errorf("isapi: keyFrameInterval is ms but fps unknown (no maxFrameRate)")
			}
			return map[string]string{"keyFrameInterval": strconv.Itoa(gopFrames * 1000 / fps)}, nil
		}
		return map[string]string{"keyFrameInterval": strconv.Itoa(gopFrames)}, nil
	}
	return nil, fmt.Errorf("isapi: no GovLength/keyFrameInterval tag in document")
}

// bitrateEdits returns the raw-XML edits to set bitrate (Kbps) and optional
// mode. When smartOn the device treats the configured bitrate as AVERAGE, so
// the average/upper cap tag is written. Mode case matches the device's current
// videoQualityControlType casing (some firmware serves lowercase vbr/cbr).
func bitrateEdits(raw []byte, smartOn bool, kbps int, mode string) (map[string]string, error) {
	edits := map[string]string{}
	cur := extractXMLString(raw, "videoQualityControlType")
	effMode := strings.ToUpper(cur)
	if effMode == "" {
		effMode = "VBR"
	}
	if mode != "" {
		if !hasXMLTag(raw, "videoQualityControlType") {
			return nil, fmt.Errorf("isapi: no videoQualityControlType tag to set mode")
		}
		v := strings.ToUpper(mode)
		if cur != "" && cur == strings.ToLower(cur) {
			v = strings.ToLower(mode)
		}
		edits["videoQualityControlType"] = v
		effMode = strings.ToUpper(mode)
	}
	val := strconv.Itoa(kbps)
	switch {
	case smartOn:
		switch {
		case hasXMLTag(raw, "vbrAverageCap"):
			edits["vbrAverageCap"] = val
		case hasXMLTag(raw, "vbrUpperCap"):
			edits["vbrUpperCap"] = val
		case hasXMLTag(raw, "constantBitRate"):
			edits["constantBitRate"] = val
		default:
			return nil, fmt.Errorf("isapi: smart codec on but no bitrate tag found")
		}
	case effMode == "CBR":
		if !hasXMLTag(raw, "constantBitRate") {
			return nil, fmt.Errorf("isapi: CBR requested but no constantBitRate tag")
		}
		edits["constantBitRate"] = val
	default: // VBR
		switch {
		case hasXMLTag(raw, "vbrUpperCap"):
			edits["vbrUpperCap"] = val
		case hasXMLTag(raw, "constantBitRate"):
			edits["constantBitRate"] = val
		default:
			return nil, fmt.Errorf("isapi: no VBR bitrate tag found")
		}
	}
	return edits, nil
}

// SetGOP sets the I-frame interval (frames) for one channel/stream, preserving
// all other device fields.
func (c *Client) SetGOP(ctx context.Context, ch, stream, gopFrames int) error {
	id := channelID(ch, stream)
	raw, err := c.do(ctx, http.MethodGet, streamPath(id), nil)
	if err != nil {
		return err
	}
	// This firmware family exposes GovLength (frames); keep the ms path off by
	// default and let gopEdits pick GovLength when present.
	edits, err := gopEdits(raw, gopFrames, false)
	if err != nil {
		return err
	}
	return c.mutateStreamChannelStrict(ctx, id, edits)
}

// SetBitrate sets the video bitrate (Kbps) and, when mode is non-empty, the
// bitrate control mode ("VBR"/"CBR") for one channel/stream, preserving all
// other device fields. When Smart Codec is on, the device treats the
// configured value as an average bitrate rather than a hard cap.
func (c *Client) SetBitrate(ctx context.Context, ch, stream, kbps int, mode string) error {
	id := channelID(ch, stream)
	raw, err := c.do(ctx, http.MethodGet, streamPath(id), nil)
	if err != nil {
		return err
	}
	smartOn := bytes.Contains(raw, []byte("<SmartCodec>")) && extractXMLInBlock(raw, "SmartCodec", "enabled") == "true"
	if !bytes.Contains(raw, []byte("<SmartCodec>")) {
		if scRes, err := c.getSmartCodec(ctx, id); err == nil {
			smartOn = scRes.Enabled
		}
	}
	edits, err := bitrateEdits(raw, smartOn, kbps, mode)
	if err != nil {
		return err
	}
	return c.mutateStreamChannelStrict(ctx, id, edits)
}

// SetResolution sets the pixel resolution (and, when fps > 0, maxFrameRate =
// fps*100) for one channel/stream, preserving all other device fields. Pass
// fps <= 0 to leave the frame rate untouched.
func (c *Client) SetResolution(ctx context.Context, ch, stream, w, h, fps int) error {
	edits := map[string]string{
		"videoResolutionWidth":  strconv.Itoa(w),
		"videoResolutionHeight": strconv.Itoa(h),
	}
	if fps > 0 {
		edits["maxFrameRate"] = strconv.Itoa(fps * 100)
	}
	return c.mutateStreamChannel(ctx, channelID(ch, stream), edits)
}

// SetFPS updates only maxFrameRate. ISAPI stores frame rate as fps*100.
func (c *Client) SetFPS(ctx context.Context, ch, stream, fps int) error {
	return c.mutateStreamChannel(ctx, channelID(ch, stream), map[string]string{
		"maxFrameRate": strconv.Itoa(fps * 100),
	})
}

var maxFrameRateTagRE = regexp.MustCompile(`(?is)<maxFrameRate\b([^>]*)>([^<]*)</maxFrameRate>`)
var numericRE = regexp.MustCompile(`[0-9]+`)

// GetMaxFPS reads the stream capability document and extracts the largest
// maxFrameRate advertised in text, opt, or max attributes. Values are fps*100.
func (c *Client) GetMaxFPS(ctx context.Context, ch, stream, width, height int, codec string) (int, error) {
	id := channelID(ch, stream)
	raw, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/ISAPI/Streaming/channels/%d/capabilities", id), nil)
	if err != nil {
		return 0, err
	}
	match := maxFrameRateTagRE.FindSubmatch(raw)
	if len(match) == 0 {
		return 0, fmt.Errorf("isapi: capabilities has no maxFrameRate")
	}
	max := 0
	for _, n := range numericRE.FindAllString(string(match[1])+" "+string(match[2]), -1) {
		v, _ := strconv.Atoi(n)
		if v > max {
			max = v
		}
	}
	if max <= 0 {
		return 0, fmt.Errorf("isapi: capabilities maxFrameRate is empty")
	}
	if max > 100 {
		max /= 100
	}
	return max, nil
}

// SetCodec sets the video codec (CodecH264/CodecH265/CodecMJPEG, or any raw
// videoCodecType the device accepts) for one channel/stream.
func (c *Client) SetCodec(ctx context.Context, ch, stream int, codec string) error {
	return c.mutateStreamChannel(ctx, channelID(ch, stream), map[string]string{
		"videoCodecType": codec,
	})
}

// SetSmartCodec toggles Smart Codec (H.264+/H.265+) for one channel/stream
// via the standalone smartCodec sub-resource. Smart Codec requires an H.265
// base codec, so when enabling it this first switches the stream's codec to
// H.265.
func (c *Client) SetSmartCodec(ctx context.Context, ch, stream int, on bool) error {
	id := channelID(ch, stream)
	if on {
		if err := c.SetCodec(ctx, ch, stream, CodecH265); err != nil {
			return fmt.Errorf("isapi: set base codec H.265 before enabling smart codec: %w", err)
		}
	}
	// Prefer the INLINE <SmartCodec><enabled> element inside the StreamingChannel
	// document: many cameras/NVR channels reject the standalone .../smartCodec
	// sub-resource with "Invalid Operation". Fall back to the sub-resource only
	// when the document has no inline SmartCodec element.
	path := streamPath(id)
	raw, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if bytes.Contains(raw, []byte("<SmartCodec>")) {
		raw = replaceXMLTagInBlock(raw, "SmartCodec", "enabled", boolStr(on))
		resp, err := c.do(ctx, http.MethodPut, path, raw)
		if err != nil {
			return err
		}
		if err := checkResponseStatus(resp); err != nil {
			return fmt.Errorf("isapi: PUT %s (inline SmartCodec): %w", path, err)
		}
		return nil
	}
	return c.putSmartCodec(ctx, id, on)
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// replaceXMLTagInBlock replaces the first <tag>…</tag> that occurs INSIDE the
// first <block>…</block> (so we edit e.g. SmartCodec's <enabled>, not the
// Video/Audio <enabled> that also appear in the document).
func replaceXMLTagInBlock(doc []byte, block, tag, value string) []byte {
	open := []byte("<" + block + ">")
	closeB := []byte("</" + block + ">")
	i := bytes.Index(doc, open)
	if i < 0 {
		return doc
	}
	rel := bytes.Index(doc[i:], closeB)
	if rel < 0 {
		return doc
	}
	end := i + rel
	seg := replaceXMLTag(doc[i:end], tag, value)
	out := make([]byte, 0, len(doc)-(end-i)+len(seg))
	out = append(out, doc[:i]...)
	out = append(out, seg...)
	out = append(out, doc[end:]...)
	return out
}

// SetAudioAAC forces the stream's audio codec to AAC, preserving other device
// fields. (Audio must already be enabled on the channel; the audio input codec
// lives in the StreamingChannel document as audioCompressionType.)
func (c *Client) SetAudioAAC(ctx context.Context, ch, stream int) error {
	return c.mutateStreamChannel(ctx, channelID(ch, stream), map[string]string{
		"audioCompressionType": "AAC",
	})
}

// GetSnapshot fetches a single JPEG frame for one channel/stream via
// GET /ISAPI/Streaming/channels/{id}/picture (confirmed against this repo's
// own docs-sdk/hikvision/hikvision-best-practices-README.md). The response
// body is the raw JPEG — no XML envelope to unmarshal.
func (c *Client) GetSnapshot(ctx context.Context, ch, stream int) ([]byte, error) {
	id := channelID(ch, stream)
	path := fmt.Sprintf("/ISAPI/Streaming/channels/%d/picture", id)
	return c.do(ctx, http.MethodGet, path, nil)
}

// inputProxyChannelPath is the ISAPI resource for one remote-IP-camera
// channel on an NVR (proxying a discrete IP camera per channel — confirmed
// live: GET returns <InputProxyChannel><name>BAN 1</name>...). ch is the
// native channel number, matching InputProxy's own <id>.
func inputProxyChannelPath(ch int) string {
	return fmt.Sprintf("/ISAPI/ContentMgmt/InputProxy/channels/%d", ch)
}

// inputProxyChannelsPath lists every InputProxy channel (all remote IP
// cameras an NVR proxies) in one GET — used by ProbeAll to fetch every
// channel's real name in a single request instead of N.
func inputProxyChannelsPath() string { return "/ISAPI/ContentMgmt/InputProxy/channels" }

// videoInputChannelPath is the ISAPI resource for one LOCAL/analog video
// input channel — the fallback source of a channel's own name on devices
// that aren't an NVR proxying remote IP cameras (a standalone IP camera, or
// an analog-input NVR). Unverified live in this codebase: every Hikvision
// device reachable during development turned out to be an InputProxy-style
// NVR, where this resource returns "Invalid Operation" (confirmed).
func videoInputChannelPath(ch int) string {
	return fmt.Sprintf("/ISAPI/System/Video/inputs/channels/%d", ch)
}

// SetChannelName writes the device's own name for a channel — tried first as
// an NVR's InputProxy remote-camera name (the common case: this repo's live
// test NVR stores "BAN 1"/"BAN 2"/... there, confirmed live; the
// StreamingChannel document's <channelName> field this package used to write
// instead just holds an internal id-like default, e.g. "101", NOT the
// operator-assigned name), falling back to the local video-input channel
// name for devices where InputProxy doesn't apply.
func (c *Client) SetChannelName(ctx context.Context, ch int, name string) error {
	path := inputProxyChannelPath(ch)
	raw, err := c.do(ctx, http.MethodGet, path, nil)
	if err == nil && hasXMLTag(raw, "name") {
		raw = replaceXMLTag(raw, "name", xmlEscaper.Replace(name))
		resp, perr := c.do(ctx, http.MethodPut, path, raw)
		if perr != nil {
			return perr
		}
		return checkResponseStatus(resp)
	}

	path = videoInputChannelPath(ch)
	raw, err = c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return fmt.Errorf("isapi: channel %d exposes neither InputProxy nor Video/inputs name: %w", ch, err)
	}
	if !hasXMLTag(raw, "name") {
		return fmt.Errorf("isapi: channel %d: no <name> tag in InputProxy or Video/inputs document", ch)
	}
	raw = replaceXMLTag(raw, "name", xmlEscaper.Replace(name))
	resp, err := c.do(ctx, http.MethodPut, path, raw)
	if err != nil {
		return err
	}
	return checkResponseStatus(resp)
}

// GetChannelName reads back the device's own name for a channel — same
// InputProxy-then-Video/inputs fallback order as SetChannelName.
func (c *Client) GetChannelName(ctx context.Context, ch int) (string, error) {
	raw, err := c.do(ctx, http.MethodGet, inputProxyChannelPath(ch), nil)
	if err == nil {
		if name := extractXMLString(raw, "name"); name != "" {
			return name, nil
		}
	}
	raw, err = c.do(ctx, http.MethodGet, videoInputChannelPath(ch), nil)
	if err != nil {
		return "", fmt.Errorf("isapi: channel %d exposes neither InputProxy nor Video/inputs name: %w", ch, err)
	}
	return extractXMLString(raw, "name"), nil
}

// inputProxyNames fetches every channel's real name in one request (the
// InputProxyChannelList document). Best-effort: returns a nil map (not an
// error) on any failure, so ProbeAll can fall back gracefully on devices
// that aren't an InputProxy-style NVR.
func (c *Client) inputProxyNames(ctx context.Context) map[int]string {
	raw, err := c.do(ctx, http.MethodGet, inputProxyChannelsPath(), nil)
	if err != nil {
		return nil
	}
	var list struct {
		Channels []struct {
			ID   string `xml:"id"`
			Name string `xml:"name"`
		} `xml:"InputProxyChannel"`
	}
	if err := xml.Unmarshal(raw, &list); err != nil {
		return nil
	}
	out := make(map[int]string, len(list.Channels))
	for _, ch := range list.Channels {
		if id, err := strconv.Atoi(ch.ID); err == nil {
			out[id] = ch.Name
		}
	}
	return out
}

// videoOverlay mirrors the subset of ISAPI's
// /ISAPI/System/Video/inputs/channels/{id}/overlays document this package
// understands: up to 4 free-text overlay lines (TextOverlayList). Field
// names follow Hikvision's standard, stable ISAPI convention but are NOT
// verified against a live device in this codebase (no ISAPI reference PDF
// shipped, and the live test camera in this project's memory is only
// reachable over the closed SDK port, not ISAPI-over-HTTP) — see
// docs/GOTCHAS.md. GetOverlayText/SetOverlayText are written defensively:
// SetOverlayText only ever writes tags it has just confirmed exist in the
// device's own document (mutateStreamChannelStrict-style raw-XML edit), so
// a wrong guess fails loudly instead of corrupting device config.
type videoOverlay struct {
	XMLName         xml.Name         `xml:"VideoOverlay"`
	TextOverlayList *textOverlayList `xml:"TextOverlayList"`
}

type textOverlayList struct {
	TextOverlay []textOverlay `xml:"TextOverlay"`
}

type textOverlay struct {
	ID          string `xml:"id"`
	Enabled     bool   `xml:"enabled"`
	DisplayText string `xml:"displayText"`
}

// overlaysPath is the ISAPI resource for a channel's on-screen text overlays.
// ch is the native (physical input) channel number — NOT the compound
// streaming-channel id used by streamPath.
func overlaysPath(ch int) string {
	return fmt.Sprintf("/ISAPI/System/Video/inputs/channels/%d/overlays", ch)
}

// ErrOverlayUnsupported is returned by GetOverlayText/SetOverlayText when the
// device's overlays document has no TextOverlayList (older firmware) or none
// of its <TextOverlay> entries carry a <displayText> tag.
var ErrOverlayUnsupported = fmt.Errorf("isapi: TextOverlayList/displayText not exposed by this device's overlays document")

// GetOverlayText reads back the free-text overlay lines currently configured
// on a channel plus each slot's on-screen enable state, in TextOverlay list
// order. Returns ErrOverlayUnsupported if the device doesn't expose
// TextOverlayList.
func (c *Client) GetOverlayText(ctx context.Context, ch int) (lines []string, enabled []bool, err error) {
	body, err := c.do(ctx, http.MethodGet, overlaysPath(ch), nil)
	if err != nil {
		return nil, nil, err
	}
	var ov videoOverlay
	if err := xml.Unmarshal(body, &ov); err != nil {
		return nil, nil, fmt.Errorf("isapi: decode overlays channel %d: %w (body: %s)", ch, err, truncate(body, 200))
	}
	if ov.TextOverlayList == nil || len(ov.TextOverlayList.TextOverlay) == 0 {
		return nil, nil, ErrOverlayUnsupported
	}
	lines = make([]string, len(ov.TextOverlayList.TextOverlay))
	enabled = make([]bool, len(ov.TextOverlayList.TextOverlay))
	for i, t := range ov.TextOverlayList.TextOverlay {
		lines[i] = t.DisplayText
		enabled[i] = t.Enabled
	}
	return lines, enabled, nil
}

// SetOverlayText writes up to the device's own number of TextOverlay slots
// worth of free-text overlay lines for a channel (extra lines beyond that
// are dropped; the count actually applied is returned), plus each slot's
// on-screen enable state. enabled[i] wins when present; a shorter enabled
// slice (or nil) falls back to enabling exactly the slots getting non-empty
// text — the old implicit behavior. Like mutateStreamChannelStrict, it edits
// the raw XML in place — replacing only <displayText>/<enabled> inside each
// Nth <TextOverlay>...</TextOverlay> block — instead of re-marshalling a
// trimmed struct, so unknown fields (id, position, color, ...) this package
// doesn't model survive the PUT unchanged. Returns ErrOverlayUnsupported if
// the device has no TextOverlayList or no <displayText> tag in it.
func (c *Client) SetOverlayText(ctx context.Context, ch int, lines []string, enabled []bool) (applied int, err error) {
	path := overlaysPath(ch)
	raw, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return 0, err
	}
	var ov videoOverlay
	if err := xml.Unmarshal(raw, &ov); err != nil {
		return 0, fmt.Errorf("isapi: decode overlays channel %d: %w (body: %s)", ch, err, truncate(raw, 200))
	}
	if ov.TextOverlayList == nil || len(ov.TextOverlayList.TextOverlay) == 0 {
		return 0, ErrOverlayUnsupported
	}
	if !bytes.Contains(raw, []byte("<displayText>")) {
		return 0, ErrOverlayUnsupported
	}
	n := len(lines)
	if slots := len(ov.TextOverlayList.TextOverlay); n > slots {
		n = slots
	}
	for i := 0; i < n; i++ {
		raw = replaceXMLTagInNthBlock(raw, "TextOverlay", i, "displayText", xmlEscaper.Replace(lines[i]))
		on := lines[i] != ""
		if i < len(enabled) {
			on = enabled[i]
		}
		raw = replaceXMLTagInNthBlock(raw, "TextOverlay", i, "enabled", boolStr(on))
	}
	resp, err := c.do(ctx, http.MethodPut, path, raw)
	if err != nil {
		return 0, err
	}
	if err := checkResponseStatus(resp); err != nil {
		return 0, fmt.Errorf("isapi: PUT %s: %w", path, err)
	}
	return n, nil
}

// replaceXMLTagInNthBlock replaces the first <tag>…</tag> inside the (0-based)
// nth occurrence of <block>…</block> in doc, leaving every other block and
// every field this package doesn't model byte-identical. Mirrors
// replaceXMLTagInBlock but for documents with a repeated block (e.g. multiple
// <TextOverlay> entries in a <TextOverlayList>).
func replaceXMLTagInNthBlock(doc []byte, block string, n int, tag, value string) []byte {
	open := []byte("<" + block + ">")
	closeB := []byte("</" + block + ">")
	pos := 0
	for i := 0; i <= n; i++ {
		rel := bytes.Index(doc[pos:], open)
		if rel < 0 {
			return doc
		}
		if i < n {
			pos += rel + len(open)
			continue
		}
		start := pos + rel
		relEnd := bytes.Index(doc[start:], closeB)
		if relEnd < 0 {
			return doc
		}
		end := start + relEnd
		seg := replaceXMLTag(doc[start:end], tag, value)
		out := make([]byte, 0, len(doc)-(end-start)+len(seg))
		out = append(out, doc[:start]...)
		out = append(out, seg...)
		out = append(out, doc[end:]...)
		return out
	}
	return doc
}

var xmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")

// SetUserPassword changes account id (1 = admin) to userName/newPass via
// PUT /ISAPI/Security/users/<id>.
func (c *Client) SetUserPassword(ctx context.Context, id int, userName, newPass string) error {
	doc := fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8"?><User><id>%d</id><userName>%s</userName><password>%s</password></User>`,
		id, xmlEscaper.Replace(userName), xmlEscaper.Replace(newPass))
	path := fmt.Sprintf("/ISAPI/Security/users/%d", id)
	resp, err := c.do(ctx, http.MethodPut, path, []byte(doc))
	if err != nil {
		return err
	}
	if err := checkResponseStatus(resp); err != nil {
		return fmt.Errorf("isapi: PUT %s: %w", path, err)
	}
	return nil
}

// streamingChannelList is /ISAPI/Streaming/channels: every channel/stream on
// the device (an NVR returns all its cameras here) in one document.
type streamingChannelList struct {
	XMLName  xml.Name           `xml:"StreamingChannelList"`
	Channels []StreamingChannel `xml:"StreamingChannel"`
}

// ProbeAll lists every channel/stream on the device in a single GET, so an
// NVR's whole camera list comes back at once. Channel is the native channel
// number (e.g. id 201 -> channel 2, stream 0).
func (c *Client) ProbeAll(ctx context.Context) ([]StreamInfo, error) {
	body, err := c.do(ctx, http.MethodGet, "/ISAPI/Streaming/channels", nil)
	if err != nil {
		return nil, err
	}
	var list streamingChannelList
	if err := xml.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("isapi: decode channel list: %w (body: %s)", err, truncate(body, 200))
	}
	// One extra request for every channel's REAL name (InputProxy's <name>,
	// e.g. "BAN 1") rather than N — <channelName> inside each StreamingChannel
	// document is just an internal id-like default ("101"), not what an
	// operator configures. Best-effort: nil on devices that aren't an
	// InputProxy-style NVR (falls back to StreamingChannel's channelName).
	names := c.inputProxyNames(ctx)
	out := make([]StreamInfo, 0, len(list.Channels))
	for _, sc := range list.Channels {
		id, _ := strconv.Atoi(sc.ID)
		if id == 0 {
			continue
		}
		channel := id / 100
		name := sc.ChannelName
		if n, ok := names[channel]; ok && n != "" {
			name = n
		}
		info := StreamInfo{Channel: channel, Stream: id%100 - 1, Name: name}
		if sc.Video != nil {
			info.Width = sc.Video.VideoResolutionWidth
			info.Height = sc.Video.VideoResolutionHeight
			if sc.Video.MaxFrameRate > 0 {
				info.FPS = sc.Video.MaxFrameRate / 100
			}
			info.Codec = sc.Video.VideoCodecType
			if sc.Video.SmartCodec != nil {
				info.SmartCodec = sc.Video.SmartCodec.Enabled
			}
			info.GOP = gopFromVideo(sc.Video)
			info.BitrateMode = strings.ToUpper(sc.Video.VideoQualityControlType)
			info.BitrateKbps = bitrateFromVideo(sc.Video, info.SmartCodec)
		}
		if sc.Audio != nil {
			info.AudioCodec = sc.Audio.AudioCompressionType
			info.AudioEnable = sc.Audio.Enabled
		}
		out = append(out, info)
	}
	return out, nil
}

// StreamInfo is a read-back summary of one stream's encode settings.
type StreamInfo struct {
	Channel     int
	Stream      int
	Width       int
	Height      int
	FPS         int
	Codec       string
	AudioCodec  string
	AudioEnable bool
	SmartCodec  bool

	GOP         int
	BitrateKbps int
	BitrateMode string

	// Name is the device's own channelName for this channel (not our
	// inventory label), and OSDLines is the on-screen text overlay content
	// when the device exposes it. Both are best-effort: populated from
	// GetChannelName/GetOverlayText, left empty if unsupported.
	Name     string
	OSDLines []string
}

// GetStreamInfo reads back the current encode settings for a channel/stream.
// Smart Codec state is read from the inline Video.SmartCodec element when
// present; otherwise it falls back to the standalone smartCodec sub-resource
// (some firmware only exposes one of the two).
func (c *Client) GetStreamInfo(ctx context.Context, ch, stream int) (StreamInfo, error) {
	id := channelID(ch, stream)
	info := StreamInfo{Channel: ch, Stream: stream}
	sc, err := c.GetStreamChannel(ctx, id)
	if err != nil {
		return info, err
	}
	// StreamingChannel's channelName is the wrong source for a real name (see
	// SetChannelName/GetChannelName) — left as a rough placeholder here on
	// purpose: this function drives the hot apply-verify before/after loop
	// (codec/resolution/GOP/bitrate), whose callers never read Name, so it's
	// not worth an extra InputProxy round-trip on every step. ProbeAll (what
	// the UI actually displays) uses the correct InputProxy-backed lookup.
	info.Name = sc.ChannelName
	if sc.Video != nil {
		info.Width = sc.Video.VideoResolutionWidth
		info.Height = sc.Video.VideoResolutionHeight
		if sc.Video.MaxFrameRate > 0 {
			info.FPS = sc.Video.MaxFrameRate / 100
		}
		info.Codec = sc.Video.VideoCodecType
		if sc.Video.SmartCodec != nil {
			info.SmartCodec = sc.Video.SmartCodec.Enabled
		}
		info.GOP = gopFromVideo(sc.Video)
		info.BitrateMode = strings.ToUpper(sc.Video.VideoQualityControlType)
	}
	if sc.Audio != nil {
		info.AudioCodec = sc.Audio.AudioCompressionType
		info.AudioEnable = sc.Audio.Enabled
	}
	if sc.Video == nil || sc.Video.SmartCodec == nil {
		if scRes, err := c.getSmartCodec(ctx, id); err == nil {
			info.SmartCodec = scRes.Enabled
		}
	}
	// Computed last so the smart-codec sub-resource fallback above (which can
	// change info.SmartCodec) is reflected in the bitrate tag precedence.
	if sc.Video != nil {
		info.BitrateKbps = bitrateFromVideo(sc.Video, info.SmartCodec)
	}
	return info, nil
}

// gopFromVideo returns the I-frame interval in frames.
func gopFromVideo(v *Video) int { return v.GovLength }

// bitrateFromVideo picks the effective bitrate (Kbps) for read-back, matching
// the tag precedence the setter writes: smart codec on -> average cap; CBR ->
// constant; VBR -> upper cap. Falls back across tags a given firmware omits.
func bitrateFromVideo(v *Video, smartOn bool) int {
	if smartOn {
		if v.VBRAverageCap > 0 {
			return v.VBRAverageCap
		}
		if v.VBRUpperCap > 0 {
			return v.VBRUpperCap
		}
		return v.ConstantBitRate
	}
	if strings.ToUpper(v.VideoQualityControlType) == "CBR" {
		if v.ConstantBitRate > 0 {
			return v.ConstantBitRate
		}
		return v.VBRUpperCap
	}
	if v.VBRUpperCap > 0 {
		return v.VBRUpperCap
	}
	return v.ConstantBitRate
}
