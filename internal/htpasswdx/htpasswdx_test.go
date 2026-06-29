package htpasswdx

import (
	"strings"
	"testing"
)

// apr1 / sha 金标准向量由 openssl 生成：
//   openssl passwd -apr1 -salt abcd1234 secret
//   printf password | openssl dgst -sha1 -binary | openssl base64
const (
	apr1Golden = "$apr1$abcd1234$KISB.4aBzP4pecxr2tTpg1"
	shaGolden  = "{SHA}W6ph5Mm5Pz8GgiULbPgzG37mj9g="
)

func TestAPR1_Vector(t *testing.T) {
	if got := APR1("secret", "abcd1234"); got != apr1Golden {
		t.Errorf("APR1 与 openssl 金标准不符\n got %q\nwant %q", got, apr1Golden)
	}
}

func TestSHA1_Vector(t *testing.T) {
	if got := SHA1("password"); got != shaGolden {
		t.Errorf("SHA1 与 openssl 金标准不符\n got %q\nwant %q", got, shaGolden)
	}
}

func TestBcrypt_RoundTrip(t *testing.T) {
	h, err := Bcrypt("hunter2", 6) // cost 6 测试够快
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$2y$") {
		t.Errorf("应输出 Apache $2y$ 前缀，got %q", h)
	}
	ok, err := Verify(h, "hunter2")
	if err != nil || !ok {
		t.Errorf("正确密码应匹配，ok=%v err=%v", ok, err)
	}
	ok, _ = Verify(h, "wrong")
	if ok {
		t.Error("错误密码不应匹配")
	}
}

func TestVerify_Formats(t *testing.T) {
	cases := []struct {
		hash, pw string
		want     bool
	}{
		{apr1Golden, "secret", true},
		{apr1Golden, "nope", false},
		{shaGolden, "password", true},
		{shaGolden, "nope", false},
	}
	for _, c := range cases {
		got, err := Verify(c.hash, c.pw)
		if err != nil {
			t.Errorf("Verify(%q,%q) err=%v", c.hash, c.pw, err)
		}
		if got != c.want {
			t.Errorf("Verify(%q,%q)=%v want %v", c.hash, c.pw, got, c.want)
		}
	}
	if _, err := Verify("not-a-known-format", "x"); err == nil {
		t.Error("未知格式应报错")
	}
}

func TestUpsert(t *testing.T) {
	content := "# comment\nalice:OLD\nbob:BHASH\n"
	// 替换同名用户
	got := Upsert(content, "alice", "alice:NEW")
	if !strings.Contains(got, "alice:NEW") || strings.Contains(got, "alice:OLD") {
		t.Errorf("alice 应被替换:\n%s", got)
	}
	if !strings.Contains(got, "# comment") || !strings.Contains(got, "bob:BHASH") {
		t.Errorf("注释和其它用户应保留:\n%s", got)
	}
	if strings.Count(got, "alice:") != 1 {
		t.Errorf("alice 不应重复:\n%s", got)
	}
	// 追加新用户
	got2 := Upsert(content, "carol", "carol:CHASH")
	if !strings.Contains(got2, "carol:CHASH") || !strings.Contains(got2, "alice:OLD") {
		t.Errorf("carol 应追加、其余保留:\n%s", got2)
	}
	// 空文件起步
	got3 := Upsert("", "dave", "dave:DHASH")
	if got3 != "dave:DHASH\n" {
		t.Errorf("空文件 upsert 应只有一行，got %q", got3)
	}
}
