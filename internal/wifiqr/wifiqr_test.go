package wifiqr

import "testing"

func TestPayload_Basic(t *testing.T) {
	got, err := Payload(Config{SSID: "MyNet", Password: "s3cr3t", Auth: AuthWPA})
	if err != nil {
		t.Fatal(err)
	}
	want := "WIFI:T:WPA;S:MyNet;P:s3cr3t;;"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

// 转义是这命令唯一的正确性关键点：逐个保留字符钉死。
func TestPayload_Escaping(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string // SSID 转义后应得到的串
	}{
		{"semicolon", "a;b", `a\;b`},
		{"colon", "a:b", `a\:b`},
		{"comma", "a,b", `a\,b`},
		{"quote", `a"b`, `a\"b`},
		{"backslash", `a\b`, `a\\b`},
		{"mixed", `a;b:c,d"e\f`, `a\;b\:c\,d\"e\\f`},
		{"none", "plain中文", "plain中文"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Payload(Config{SSID: c.in, Password: "x", Auth: AuthWPA})
			if err != nil {
				t.Fatal(err)
			}
			want := "WIFI:T:WPA;S:" + c.want + ";P:x;;"
			if got != want {
				t.Errorf("SSID %q → %q, want %q", c.in, got, want)
			}
		})
	}
}

func TestPayload_Nopass(t *testing.T) {
	got, err := Payload(Config{SSID: "Open", Auth: AuthNopass, Password: "ignored"})
	if err != nil {
		t.Fatal(err)
	}
	want := "WIFI:T:nopass;S:Open;;" // 不应出现 P:
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestPayload_Hidden(t *testing.T) {
	got, _ := Payload(Config{SSID: "H", Password: "x", Auth: AuthWPA, Hidden: true})
	want := "WIFI:T:WPA;S:H;P:x;H:true;;"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestPayload_DefaultAuthWPA(t *testing.T) {
	got, _ := Payload(Config{SSID: "N", Password: "x"}) // Auth 空 → WPA
	if got != "WIFI:T:WPA;S:N;P:x;;" {
		t.Errorf("空 Auth 应默认 WPA，got %q", got)
	}
}

func TestPayload_EmptySSID(t *testing.T) {
	if _, err := Payload(Config{SSID: "  ", Password: "x"}); err == nil {
		t.Error("空 SSID 应报错")
	}
}

func TestParseAuth(t *testing.T) {
	ok := map[string]Auth{
		"wpa": AuthWPA, "WPA": AuthWPA, "wpa2": AuthWPA, "wpa3": AuthWPA, "": AuthWPA,
		"wep": AuthWEP,
		"nopass": AuthNopass, "open": AuthNopass, "none": AuthNopass,
	}
	for in, want := range ok {
		got, err := ParseAuth(in)
		if err != nil || got != want {
			t.Errorf("ParseAuth(%q)=%q,%v want %q", in, got, err, want)
		}
	}
	if _, err := ParseAuth("wpa-enterprise"); err == nil {
		t.Error("未知认证类型应报错")
	}
}
