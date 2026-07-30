package dahua

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ngohuynhngockhanh/ksp-camera-auto/internal/isapi"
)

const timeConfigLayout = "2006-01-02 15:04:05"

type TimeConfig struct {
	CurrentTime  string `json:"currentTime"`
	NTPEnable    bool   `json:"ntpEnable"`
	NTPAddress   string `json:"ntpAddress"`
	NTPPort      int    `json:"ntpPort"`
	UpdatePeriod int    `json:"updatePeriod"`
	TimeZone     int    `json:"timeZone"`
	TimeZoneDesc string `json:"timeZoneDesc"`
}

func dahuaCGI(ctx context.Context, host, user, pass, path string, q url.Values) (string, error) {
	u := url.URL{Scheme: "http", Host: host, Path: path, RawQuery: q.Encode()}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Transport: isapi.NewDigestTransport(user, pass, nil)}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	body := strings.TrimSpace(string(b))
	if resp.StatusCode/100 != 2 || strings.HasPrefix(body, "Error") {
		return "", fmt.Errorf("dahua CGI %s: HTTP %d: %s", path, resp.StatusCode, body)
	}
	return body, nil
}

func parseConfigLines(body string) map[string]string {
	out := map[string]string{}
	s := bufio.NewScanner(strings.NewReader(body))
	for s.Scan() {
		k, v, ok := strings.Cut(strings.TrimSpace(s.Text()), "=")
		if ok {
			out[strings.TrimPrefix(k, "table.NTP.")] = strings.TrimSpace(v)
		}
	}
	return out
}

func GetTimeConfig(ctx context.Context, host, user, pass string) (TimeConfig, error) {
	clock, err := dahuaCGI(ctx, host, user, pass, "/cgi-bin/global.cgi", url.Values{"action": {"getCurrentTime"}})
	if err != nil {
		return TimeConfig{}, err
	}
	ntp, err := dahuaCGI(ctx, host, user, pass, "/cgi-bin/configManager.cgi", url.Values{"action": {"getConfig"}, "name": {"NTP"}})
	if err != nil {
		return TimeConfig{}, err
	}
	v := parseConfigLines(ntp)
	port, _ := strconv.Atoi(v["Port"])
	period, _ := strconv.Atoi(v["UpdatePeriod"])
	tz, _ := strconv.Atoi(v["TimeZone"])
	return TimeConfig{
		CurrentTime:  strings.TrimPrefix(clock, "result="),
		NTPEnable:    strings.EqualFold(v["Enable"], "true"),
		NTPAddress:   v["Address"],
		NTPPort:      port,
		UpdatePeriod: period,
		TimeZone:     tz,
		TimeZoneDesc: v["TimeZoneDesc"],
	}, nil
}

func SetTimeConfig(ctx context.Context, host, user, pass string, cfg TimeConfig) error {
	if cfg.CurrentTime != "" {
		if _, err := time.ParseInLocation(timeConfigLayout, cfg.CurrentTime, time.Local); err != nil {
			return fmt.Errorf("invalid device datetime: %w", err)
		}
		if _, err := dahuaCGI(ctx, host, user, pass, "/cgi-bin/global.cgi", url.Values{"action": {"setCurrentTime"}, "time": {cfg.CurrentTime}}); err != nil {
			return err
		}
	}
	if cfg.NTPAddress == "" {
		cfg.NTPAddress = "time.google.com"
	}
	if cfg.NTPPort == 0 {
		cfg.NTPPort = 123
	}
	if cfg.UpdatePeriod <= 0 {
		cfg.UpdatePeriod = 60
	}
	q := url.Values{"action": {"setConfig"}}
	q.Set("NTP.Enable", strconv.FormatBool(cfg.NTPEnable))
	q.Set("NTP.Address", cfg.NTPAddress)
	q.Set("NTP.Port", strconv.Itoa(cfg.NTPPort))
	q.Set("NTP.UpdatePeriod", strconv.Itoa(cfg.UpdatePeriod))
	q.Set("NTP.TimeZone", strconv.Itoa(cfg.TimeZone))
	if cfg.TimeZoneDesc != "" {
		q.Set("NTP.TimeZoneDesc", cfg.TimeZoneDesc)
	}
	_, err := dahuaCGI(ctx, host, user, pass, "/cgi-bin/configManager.cgi", q)
	return err
}
