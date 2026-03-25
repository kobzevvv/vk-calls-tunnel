package turn

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

const (
	// VK Calls app credentials (public, embedded in VK web client)
	vkClientID     = "6287487"
	vkClientSecret = "QbYic1K3lEV5kTGiqlq2"
	// OK.ru SDK application key
	okAppKey = "CGMMEJLGDIHBABABA"
	// Browser User-Agent (required by VK)
	userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:144.0) Gecko/20100101 Firefox/144.0"
)

// Credentials holds TURN server authentication data.
type Credentials struct {
	Username    string
	Password    string
	TURNServers []string // IP:port list
}

// FetchFromLink extracts TURN credentials from a VK call link.
// 6-step anonymous OAuth chain through VK API and OK.ru.
func FetchFromLink(vkLink string) (*Credentials, error) {
	// Step 1: Get anonymous token
	anonToken, err := getAnonToken("", "")
	if err != nil {
		return nil, fmt.Errorf("step 1 (anon token): %w", err)
	}

	// Step 2: Get anonymous access token payload
	payload, err := getAnonAccessTokenPayload(anonToken)
	if err != nil {
		return nil, fmt.Errorf("step 2 (access token payload): %w", err)
	}

	// Step 3: Get messages-scoped anonymous token
	msgToken, err := getAnonToken("messages", payload)
	if err != nil {
		return nil, fmt.Errorf("step 3 (messages token): %w", err)
	}

	// Step 4: Get call anonymous token
	callToken, err := getCallAnonToken(msgToken, vkLink)
	if err != nil {
		return nil, fmt.Errorf("step 4 (call token): %w", err)
	}

	// Step 5: OK.ru anonymous login
	sessionKey, err := okAnonymousLogin()
	if err != nil {
		return nil, fmt.Errorf("step 5 (ok.ru login): %w", err)
	}

	// Step 6: Join call via OK.ru — get TURN credentials
	creds, err := joinCallOK(sessionKey, callToken, vkLink)
	if err != nil {
		return nil, fmt.Errorf("step 6 (join call): %w", err)
	}

	return creds, nil
}

// FetchFromManual creates credentials for a manually specified TURN server.
func FetchFromManual(turnAddr string, username, password string) *Credentials {
	return &Credentials{
		Username:    username,
		Password:    password,
		TURNServers: []string{turnAddr},
	}
}

// postForm sends a POST with User-Agent header.
func postForm(reqURL string, data url.Values) (*http.Response, error) {
	req, err := http.NewRequest("POST", reqURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	return http.DefaultClient.Do(req)
}

// Step 1 & 3: Get anonymous token from login.vk.ru
func getAnonToken(tokenType, payload string) (string, error) {
	params := url.Values{
		"client_id":     {vkClientID},
		"client_secret": {vkClientSecret},
		"app_id":        {vkClientID},
		"version":       {"1"},
	}
	if tokenType != "" {
		params.Set("token_type", tokenType)
	}
	if payload != "" {
		params.Set("payload", payload)
	}

	resp, err := postForm("https://login.vk.ru/?act=get_anonym_token", params)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Type string `json:"type"`
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := decodeJSON(resp.Body, &result); err != nil {
		return "", err
	}
	if result.Data.AccessToken == "" {
		return "", fmt.Errorf("empty token (type: %s)", result.Type)
	}
	return result.Data.AccessToken, nil
}

// Step 2: Get anonymous access token payload
func getAnonAccessTokenPayload(anonToken string) (string, error) {
	params := url.Values{
		"access_token": {anonToken},
	}

	resp, err := postForm(
		"https://api.vk.ru/method/calls.getAnonymousAccessTokenPayload?v=5.264&client_id="+vkClientID,
		params,
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Response struct {
			Payload string `json:"payload"`
		} `json:"response"`
		Error *vkError `json:"error"`
	}
	if err := decodeJSON(resp.Body, &result); err != nil {
		return "", err
	}
	if result.Error != nil {
		return "", fmt.Errorf("VK API: [%d] %s", result.Error.Code, result.Error.Message)
	}
	if result.Response.Payload == "" {
		return "", fmt.Errorf("empty payload")
	}
	return result.Response.Payload, nil
}

// Step 4: Get call anonymous token
func getCallAnonToken(msgToken, vkLink string) (string, error) {
	params := url.Values{
		"access_token": {msgToken},
		"vk_join_link": {vkLink},
		"name":         {"123"},
	}

	resp, err := postForm("https://api.vk.ru/method/calls.getAnonymousToken?v=5.264", params)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Response struct {
			Token string `json:"token"`
		} `json:"response"`
		Error *vkError `json:"error"`
	}
	if err := decodeJSON(resp.Body, &result); err != nil {
		return "", err
	}
	if result.Error != nil {
		return "", fmt.Errorf("VK API: [%d] %s", result.Error.Code, result.Error.Message)
	}
	if result.Response.Token == "" {
		return "", fmt.Errorf("empty call token")
	}
	return result.Response.Token, nil
}

// Step 5: OK.ru anonymous login (no VK token needed, creates a fresh session)
func okAnonymousLogin() (string, error) {
	deviceID := uuid.New().String()
	sessionData := fmt.Sprintf(
		`{"version":2,"device_id":"%s","client_version":1.1,"client_type":"SDK_JS"}`,
		deviceID,
	)

	params := url.Values{
		"method":          {"auth.anonymLogin"},
		"format":          {"JSON"},
		"application_key": {okAppKey},
		"session_data":    {sessionData},
	}

	resp, err := postForm("https://calls.okcdn.ru/fb.do", params)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		SessionKey string `json:"session_key"`
		ErrorMsg   string `json:"error_msg"`
	}
	if err := decodeJSON(resp.Body, &result); err != nil {
		return "", err
	}
	if result.ErrorMsg != "" {
		return "", fmt.Errorf("OK.ru: %s", result.ErrorMsg)
	}
	if result.SessionKey == "" {
		return "", fmt.Errorf("empty session key")
	}
	return result.SessionKey, nil
}

var callHashRegexp = regexp.MustCompile(`/call/join/([A-Za-z0-9_-]+)`)

// Step 6: Join call to get TURN credentials
func joinCallOK(sessionKey, callToken, vkLink string) (*Credentials, error) {
	// OK.ru expects just the hash, not the full URL
	matches := callHashRegexp.FindStringSubmatch(vkLink)
	if len(matches) < 2 {
		return nil, fmt.Errorf("cannot extract call hash from: %s", vkLink)
	}
	callHash := matches[1]

	params := url.Values{
		"method":          {"vchat.joinConversationByLink"},
		"format":          {"JSON"},
		"application_key": {okAppKey},
		"session_key":     {sessionKey},
		"anonymToken":     {callToken},
		"joinLink":        {callHash},
		"isVideo":         {"false"},
		"protocolVersion": {"5"},
	}

	resp, err := postForm("https://calls.okcdn.ru/fb.do", params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Response contains turn_server with username, credential, urls
	var result struct {
		TURNServer *struct {
			URLs       interface{} `json:"urls"`
			Username   string      `json:"username"`
			Credential string      `json:"credential"`
		} `json:"turn_server"`
		ErrorMsg string `json:"error_msg"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w (body: %s)", err, string(body))
	}

	if result.ErrorMsg != "" {
		return nil, fmt.Errorf("OK.ru: %s", result.ErrorMsg)
	}

	if result.TURNServer == nil {
		return nil, fmt.Errorf("no turn_server in response: %s", string(body))
	}

	creds := &Credentials{
		Username: result.TURNServer.Username,
		Password: result.TURNServer.Credential,
	}

	for _, u := range extractURLs(result.TURNServer.URLs) {
		addr := parseTURNURL(u)
		if addr != "" {
			creds.TURNServers = append(creds.TURNServers, addr)
		}
	}

	if len(creds.TURNServers) == 0 {
		return nil, fmt.Errorf("no TURN server URLs in response: %s", string(body))
	}

	return creds, nil
}

func extractURLs(v interface{}) []string {
	switch urls := v.(type) {
	case string:
		return []string{urls}
	case []interface{}:
		var result []string
		for _, u := range urls {
			if s, ok := u.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

func parseTURNURL(turnURL string) string {
	addr := turnURL
	addr = strings.TrimPrefix(addr, "turns:")
	addr = strings.TrimPrefix(addr, "turn:")
	if idx := strings.Index(addr, "?"); idx != -1 {
		addr = addr[:idx]
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if !strings.Contains(addr, ":") {
		addr += ":3478"
	}
	return addr
}

type vkError struct {
	Code    int    `json:"error_code"`
	Message string `json:"error_msg"`
}

func decodeJSON(r io.Reader, v interface{}) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
}
