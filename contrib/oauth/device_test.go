package oauth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"
)

func TestDeviceClientPublicFlowAndPollingControl(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	var tokenCalls int
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if r.Header.Get("Authorization") != "" || r.Form.Get("client_secret") != "" || r.Form.Get("client_id") != "device-client" {
			t.Errorf("public client request = headers %v form %v", r.Header, r.Form)
		}
		switch r.URL.Path {
		case "/device":
			if r.Form.Get("scope") != "openid profile" {
				t.Errorf("scope = %q", r.Form.Get("scope"))
			}
			_, _ = io.WriteString(w, `{"device_code":"secret-device-code","user_code":"BCDF-GHJK","verification_uri":"https://issuer.example/verify","verification_uri_complete":"https://issuer.example/verify?user_code=BCDF-GHJK","expires_in":600,"interval":5}`)
		case "/token":
			tokenCalls++
			if r.Form.Get("grant_type") != DeviceGrantType || r.Form.Get("device_code") != "secret-device-code" {
				t.Errorf("token form = %v", r.Form)
			}
			switch tokenCalls {
			case 1:
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":"authorization_pending"}`)
			case 2:
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":"slow_down"}`)
			default:
				_, _ = io.WriteString(w, `{"access_token":"access","token_type":"Bearer"}`)
			}
		default:
			http.NotFound(w, r)
		}
	})
	var waits []time.Duration
	client, err := NewDeviceClient(DeviceConfig{
		DeviceAuthorizationEndpoint: "https://issuer.example/device",
		TokenEndpoint:               "https://issuer.example/token", ClientID: "device-client",
	}, DeviceOptions{Clock: func() time.Time { return now }, HTTPClient: &http.Client{Transport: roundTripHandler{handler: handler}}, Wait: func(_ context.Context, duration time.Duration) error {
		waits = append(waits, duration)
		now = now.Add(duration)
		return nil
	}})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	authorization, err := client.Begin(t.Context(), DeviceBeginOptions{Scopes: []string{"openid", "profile"}})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if authorization.deviceCode != "secret-device-code" || authorization.Interval != 5 || authorization.ExpiresAt != time.Unix(1_700_000_600, 0) {
		t.Fatalf("authorization = %#v", authorization)
	}
	set, err := client.Poll(t.Context(), authorization)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if set.AccessToken != "access" || !reflect.DeepEqual(waits, []time.Duration{5 * time.Second, 5 * time.Second, 10 * time.Second}) {
		t.Fatalf("set=%#v waits=%v", set, waits)
	}
}

func TestDeviceClientTerminalErrors(t *testing.T) {
	for code, want := range map[string]error{"access_denied": ErrAccessDenied, "expired_token": ErrExpired} {
		now := time.Unix(100, 0)
		client, err := NewDeviceClient(DeviceConfig{DeviceAuthorizationEndpoint: "https://issuer.example/device", TokenEndpoint: "https://issuer.example/token", ClientID: "public"}, DeviceOptions{
			Clock: func() time.Time { return now }, Wait: func(_ context.Context, duration time.Duration) error { now = now.Add(duration); return nil },
			HTTPClient: &http.Client{Transport: roundTripHandler{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = io.WriteString(w, `{"error":"`+code+`"}`)
			})}},
		})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Poll(t.Context(), DeviceAuthorization{deviceCode: "code", ExpiresIn: 60, Interval: 5, ExpiresAt: now.Add(time.Minute)})
		if !errors.Is(err, want) {
			t.Fatalf("%s error = %v, want %v", code, err, want)
		}
	}
}

func TestDeviceClientConfidentialAuthenticationMethods(t *testing.T) {
	for _, method := range []string{AuthBasic, AuthPost} {
		t.Run(method, func(t *testing.T) {
			now := time.Unix(100, 0)
			calls := 0
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				_ = r.ParseForm()
				id, secret, basic := r.BasicAuth()
				if method == AuthBasic && (!basic || id != "client" || secret != "secret" || r.Form.Get("client_secret") != "") {
					t.Errorf("basic authentication = %q/%q/%v form=%v", id, secret, basic, r.Form)
				}
				if method == AuthPost && (basic || r.Form.Get("client_secret") != "secret") {
					t.Errorf("post authentication = basic:%v form=%v", basic, r.Form)
				}
				if calls == 1 {
					_, _ = io.WriteString(w, `{"device_code":"code","user_code":"BCDF-GHJK","verification_uri":"https://issuer.example/verify","expires_in":60,"interval":5}`)
					return
				}
				_, _ = io.WriteString(w, `{"access_token":"access","token_type":"Bearer"}`)
			})
			client, err := NewDeviceClient(DeviceConfig{DeviceAuthorizationEndpoint: "https://issuer.example/device", TokenEndpoint: "https://issuer.example/token",
				ClientID: "client", ClientSecret: "secret", AuthMethod: method}, DeviceOptions{Clock: func() time.Time { return now },
				Wait: func(_ context.Context, duration time.Duration) error { now = now.Add(duration); return nil }, HTTPClient: &http.Client{Transport: roundTripHandler{handler: handler}}})
			if err != nil {
				t.Fatal(err)
			}
			authorization, err := client.Begin(t.Context(), DeviceBeginOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.Poll(t.Context(), authorization); err != nil {
				t.Fatal(err)
			}
		})
	}
}
