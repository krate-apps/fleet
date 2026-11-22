package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	myjdAPIRoot = "https://api.jdownloader.org"
	myjdAppKey  = "org.fleet.jdownloader"
	myjdTimeout = 20 * time.Second
)

type myjdClient struct {
	mu sync.Mutex

	email    string
	password string

	rid           int64
	loginSecret   []byte
	deviceSecret  []byte
	sessionToken  string
	regainToken   string
	serverEncTok  []byte
	deviceEncTok  []byte
	devices       []myjdDevice
	httpClient    *http.Client
}

type myjdDevice struct {
	Name string `json:"name"`
	ID   string `json:"id"`
	Type string `json:"type"`
}

func newMyJD(email, password string) *myjdClient {
	return &myjdClient{
		email:      strings.TrimSpace(email),
		password:   password,
		rid:        time.Now().UnixMilli(),
		httpClient: &http.Client{Timeout: myjdTimeout},
	}
}

func (c *myjdClient) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if strings.TrimSpace(c.email) == "" || c.password == "" {
		return fmt.Errorf("email and password required")
	}
	c.loginSecret = myjdSecret(c.email, c.password, "server")
	c.deviceSecret = myjdSecret(c.email, c.password, "device")
	c.sessionToken = ""
	c.regainToken = ""
	c.serverEncTok = nil
	c.deviceEncTok = nil
	c.devices = nil

	raw, err := c.requestGETLocked("/my/connect", url.Values{
		"email":  {c.email},
		"appkey": {myjdAppKey},
	}, true)
	if err != nil {
		return err
	}
	var resp struct {
		SessionToken string `json:"sessiontoken"`
		RegainToken  string `json:"regaintoken"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("connect decode: %w", err)
	}
	if resp.SessionToken == "" {
		return fmt.Errorf("connect: empty session token")
	}
	c.sessionToken = resp.SessionToken
	c.regainToken = resp.RegainToken
	c.updateEncTokensLocked()
	return c.listDevicesLocked()
}

func (c *myjdClient) ensureConnected() error {
	c.mu.Lock()
	ok := c.sessionToken != "" && c.deviceEncTok != nil
	c.mu.Unlock()
	if ok {
		return nil
	}
	return c.connect()
}

func (c *myjdClient) listDevicesLocked() error {
	raw, err := c.requestGETLocked("/my/listdevices", url.Values{
		"sessiontoken": {c.sessionToken},
	}, false)
	if err != nil {
		return err
	}
	var resp struct {
		List []myjdDevice `json:"list"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("listdevices decode: %w", err)
	}
	c.devices = resp.List
	return nil
}

func (c *myjdClient) resolveDeviceID(deviceName string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.devices) == 0 {
		if err := c.listDevicesLocked(); err != nil {
			return "", err
		}
	}
	want := strings.TrimSpace(deviceName)
	if want == "" {
		if len(c.devices) == 0 {
			return "", fmt.Errorf("no MyJD devices online")
		}
		return c.devices[0].ID, nil
	}
	for _, d := range c.devices {
		if strings.EqualFold(d.Name, want) {
			return d.ID, nil
		}
	}
	names := make([]string, 0, len(c.devices))
	for _, d := range c.devices {
		names = append(names, d.Name)
	}
	if len(names) == 0 {
		return "", fmt.Errorf("device %q not found (no devices online)", want)
	}
	return "", fmt.Errorf("device %q not found (available: %s)", want, strings.Join(names, ", "))
}

func (c *myjdClient) deviceAction(deviceID, path string, params []any) (json.RawMessage, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	raw, err := c.requestDevicePOSTLocked(deviceID, path, params)
	c.mu.Unlock()
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "token") {
		if cerr := c.connect(); cerr != nil {
			return nil, err
		}
		c.mu.Lock()
		raw, err = c.requestDevicePOSTLocked(deviceID, path, params)
		c.mu.Unlock()
	}
	return raw, err
}

func (c *myjdClient) requestGETLocked(path string, q url.Values, preAuth bool) (json.RawMessage, error) {
	c.rid++
	q.Set("rid", strconv.FormatInt(c.rid, 10))
	query := path + "?" + q.Encode()
	sigKey := c.loginSecret
	if !preAuth {
		sigKey = c.serverEncTok
	}
	sig := myjdHMAC(sigKey, query)
	full := myjdAPIRoot + query + "&signature=" + sig

	resp, err := c.httpClient.Get(full)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("MyJD HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	tok := c.loginSecret
	if !preAuth {
		tok = c.serverEncTok
	}
	plain, err := myjdDecrypt(tok, string(body))
	if err != nil {
		return nil, err
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(plain, &envelope); err != nil {
		return nil, fmt.Errorf("MyJD JSON: %w", err)
	}
	if ridRaw, ok := envelope["rid"]; ok {
		var rid int64
		_ = json.Unmarshal(ridRaw, &rid)
		if rid != 0 && rid != c.rid {
			return nil, fmt.Errorf("MyJD rid mismatch")
		}
	}
	return plain, nil
}

func (c *myjdClient) requestDevicePOSTLocked(deviceID, path string, params []any) (json.RawMessage, error) {
	c.rid++
	adapted := adaptParams(params)
	payload := map[string]any{
		"apiVer": 1,
		"url":    path,
		"params": adapted,
		"rid":    c.rid,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	enc, err := myjdEncrypt(c.deviceEncTok, data)
	if err != nil {
		return nil, err
	}
	actionURL := fmt.Sprintf("%s/t_%s_%s%s", myjdAPIRoot, c.sessionToken, deviceID, path)
	req, err := http.NewRequest(http.MethodPost, actionURL, bytes.NewReader([]byte(enc)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/aesjson-jd; charset=utf-8")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		msg := string(body)
		if plain, derr := myjdDecrypt(c.deviceEncTok, msg); derr == nil {
			msg = string(plain)
		}
		return nil, fmt.Errorf("MyJD device HTTP %d: %s", resp.StatusCode, truncate(msg, 240))
	}
	plain, err := myjdDecrypt(c.deviceEncTok, string(body))
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
		RID  int64           `json:"rid"`
	}
	if err := json.Unmarshal(plain, &envelope); err != nil {
		return nil, fmt.Errorf("MyJD device JSON: %w", err)
	}
	if envelope.RID != 0 && envelope.RID != c.rid {
		return nil, fmt.Errorf("MyJD rid mismatch")
	}
	if len(envelope.Data) == 0 {
		return json.RawMessage("null"), nil
	}
	return envelope.Data, nil
}

func (c *myjdClient) updateEncTokensLocked() {
	sessionHex, err := hex.DecodeString(c.sessionToken)
	if err != nil {
		return
	}
	h := sha256.New()
	old := c.loginSecret
	if c.serverEncTok != nil {
		old = c.serverEncTok
	}
	h.Write(old)
	h.Write(sessionHex)
	c.serverEncTok = h.Sum(nil)

	h = sha256.New()
	h.Write(c.deviceSecret)
	h.Write(sessionHex)
	c.deviceEncTok = h.Sum(nil)
}

func myjdSecret(email, password, domain string) []byte {
	h := sha256.New()
	h.Write([]byte(strings.ToLower(email)))
	h.Write([]byte(password))
	h.Write([]byte(strings.ToLower(domain)))
	return h.Sum(nil)
}

func myjdHMAC(key []byte, data string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func myjdEncrypt(secret []byte, data []byte) (string, error) {
	if len(secret) < 32 {
		return "", fmt.Errorf("invalid encryption token")
	}
	iv := secret[:16]
	key := secret[16:32]
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	padded := pkcs7Pad(data, aes.BlockSize)
	out := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, padded)
	return base64.StdEncoding.EncodeToString(out), nil
}

func myjdDecrypt(secret []byte, data string) ([]byte, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("invalid encryption token")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(data))
	if err != nil {
		return nil, err
	}
	iv := secret[:16]
	key := secret[16:32]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(raw)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext length")
	}
	out := make([]byte, len(raw))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, raw)
	return pkcs7Unpad(out)
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	pad := blockSize - len(data)%blockSize
	out := make([]byte, len(data)+pad)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(pad)
	}
	return out
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty plaintext")
	}
	pad := int(data[len(data)-1])
	if pad < 1 || pad > aes.BlockSize || pad > len(data) {
		return nil, fmt.Errorf("bad PKCS7 padding")
	}
	for i := 0; i < pad; i++ {
		if data[len(data)-1-i] != byte(pad) {
			return nil, fmt.Errorf("bad PKCS7 padding")
		}
	}
	return data[:len(data)-pad], nil
}

func adaptParams(params []any) []any {
	if params == nil {
		return nil
	}
	out := make([]any, 0, len(params))
	for _, p := range params {
		switch v := p.(type) {
		case map[string]any:
			b, _ := json.Marshal(v)
			out = append(out, string(b))
		case []any:
			out = append(out, adaptParams(v))
		default:
			out = append(out, v)
		}
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
