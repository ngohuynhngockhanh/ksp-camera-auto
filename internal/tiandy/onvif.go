package tiandy

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/dahua"
)

// onvifDeviceURL is the ONVIF device service endpoint on a Tiandy NVR.
func (c *Client) onvifDeviceURL() string {
	return fmt.Sprintf("http://%s:%d/onvif/device_service", c.host, onvifPort)
}

// soapCall POSTs a SOAP body to the ONVIF device service. When auth is true it
// prepends a WS-Security UsernameToken with a PasswordDigest, synchronising the
// Created timestamp to the device's own clock (fetched unauthenticated) so a
// skewed local clock can't invalidate the token.
func (c *Client) soapCall(ctx context.Context, body string, auth bool) (string, error) {
	header := ""
	if auth {
		created := c.deviceUTC(ctx)
		nonce := make([]byte, 16)
		if _, err := rand.Read(nonce); err != nil {
			return "", err
		}
		// PasswordDigest = base64(sha1(nonce_raw + created + password)).
		h := sha1.New()
		h.Write(nonce)
		h.Write([]byte(created))
		h.Write([]byte(c.pass))
		digest := base64.StdEncoding.EncodeToString(h.Sum(nil))
		header = fmt.Sprintf(`<s:Header><wsse:Security xmlns:wsse="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-secext-1.0.xsd" xmlns:wsu="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-wssecurity-utility-1.0.xsd"><wsse:UsernameToken><wsse:Username>%s</wsse:Username><wsse:Password Type="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-username-token-profile-1.0#PasswordDigest">%s</wsse:Password><wsse:Nonce EncodingType="http://docs.oasis-open.org/wss/2004/01/oasis-200401-wss-soap-message-security-1.0#Base64Binary">%s</wsse:Nonce><wsu:Created>%s</wsu:Created></wsse:UsernameToken></wsse:Security></s:Header>`,
			xmlEsc(c.user), digest, base64.StdEncoding.EncodeToString(nonce), created)
	}
	env := `<?xml version="1.0" encoding="utf-8"?><s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">` +
		header +
		`<s:Body xmlns:tds="http://www.onvif.org/ver10/device/wsdl">` + body + `</s:Body></s:Envelope>`

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.onvifDeviceURL(), strings.NewReader(env))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
	resp, err := (&http.Client{Timeout: c.timeout}).Do(req)
	if err != nil {
		return "", fmt.Errorf("tiandy: onvif %s: %w", c.host, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// deviceUTC fetches the device's current UTC time (unauthenticated) formatted
// as an ISO-8601 "Z" string, for use as the UsernameToken Created value. On any
// failure it falls back to the local clock.
func (c *Client) deviceUTC(ctx context.Context) string {
	resp, err := c.soapCall(ctx, `<tds:GetSystemDateAndTime/>`, false)
	if err == nil {
		var env struct {
			UTC struct {
				Time struct{ Hour, Minute, Second int }
				Date struct{ Year, Month, Day int }
			} `xml:"Body>GetSystemDateAndTimeResponse>SystemDateAndTime>UTCDateTime"`
		}
		if xml.Unmarshal([]byte(resp), &env) == nil && env.UTC.Date.Year > 0 {
			d := env.UTC
			return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02dZ",
				d.Date.Year, d.Date.Month, d.Date.Day, d.Time.Hour, d.Time.Minute, d.Time.Second)
		}
	}
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

// GetNetworkConfig reads the device's network interfaces over ONVIF and maps
// them onto the shared dahua.NetworkConfig shape, so /api/network and the web
// UI render Tiandy IP config with no vendor-specific branch. Read-only: writes
// go through SetStaticIP, which is unsupported for Tiandy.
//
// Returns ErrONVIFUnauthorized when the device rejects the credentials (ONVIF
// commonly uses a separate user account from web-admin).
func (c *Client) GetNetworkConfig(ctx context.Context) (dahua.NetworkConfig, error) {
	resp, err := c.soapCall(ctx, `<tds:GetNetworkInterfaces/>`, true)
	if err != nil {
		return dahua.NetworkConfig{}, err
	}
	if strings.Contains(resp, "NotAuthorized") || strings.Contains(resp, "not Authorized") {
		return dahua.NetworkConfig{}, ErrONVIFUnauthorized
	}

	var env struct {
		Ifaces []struct {
			Token string `xml:"token,attr"`
			Info  struct {
				Name   string `xml:"Name"`
				HwAddr string `xml:"HwAddress"`
				MTU    int    `xml:"MTU"`
			} `xml:"Info"`
			IPv4 struct {
				Config struct {
					DHCP   bool `xml:"DHCP"`
					Manual []struct {
						Address      string `xml:"Address"`
						PrefixLength int    `xml:"PrefixLength"`
					} `xml:"Manual"`
					FromDHCP struct {
						Address      string `xml:"Address"`
						PrefixLength int    `xml:"PrefixLength"`
					} `xml:"FromDHCP"`
				} `xml:"Config"`
			} `xml:"IPv4"`
		} `xml:"Body>GetNetworkInterfacesResponse>NetworkInterfaces"`
	}
	if err := xml.Unmarshal([]byte(resp), &env); err != nil {
		return dahua.NetworkConfig{}, fmt.Errorf("tiandy: parse network interfaces: %w", err)
	}

	cfg := dahua.NetworkConfig{Interfaces: map[string]map[string]any{}}
	for i, ni := range env.Ifaces {
		key := ni.Token
		if key == "" {
			key = fmt.Sprintf("eth%d", i)
		}
		if i == 0 {
			cfg.DefaultInterface = key
		}
		addr, prefix := "", 0
		if len(ni.IPv4.Config.Manual) > 0 {
			addr, prefix = ni.IPv4.Config.Manual[0].Address, ni.IPv4.Config.Manual[0].PrefixLength
		} else if ni.IPv4.Config.FromDHCP.Address != "" {
			addr, prefix = ni.IPv4.Config.FromDHCP.Address, ni.IPv4.Config.FromDHCP.PrefixLength
		}
		cfg.Interfaces[key] = map[string]any{
			"DhcpEnable":      ni.IPv4.Config.DHCP,
			"IPAddress":       addr,
			"SubnetMask":      prefixToMask(prefix),
			"PhysicalAddress": ni.Info.HwAddr,
			"MTU":             ni.Info.MTU,
		}
	}
	return cfg, nil
}

// prefixToMask renders an IPv4 prefix length (e.g. 24) as a dotted-decimal
// netmask ("255.255.255.0"). Returns "" for an out-of-range prefix.
func prefixToMask(prefix int) string {
	if prefix < 0 || prefix > 32 {
		return ""
	}
	var mask uint32 = 0xffffffff << (32 - uint(prefix))
	if prefix == 0 {
		mask = 0
	}
	return fmt.Sprintf("%d.%d.%d.%d", byte(mask>>24), byte(mask>>16), byte(mask>>8), byte(mask))
}

func xmlEsc(s string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(s))
	return b.String()
}
