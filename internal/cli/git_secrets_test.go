package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/xunull/jdan/internal/gitx"
)

// fakeGitleaks 记录被调用的参数，返回预设报告/错误。
type fakeGitleaks struct {
	report  string
	leaks   bool
	err     error
	gotDir  string
	gotArgs []string
}

func (f *fakeGitleaks) fn(dir string, args []string) (string, bool, error) {
	f.gotDir = dir
	f.gotArgs = args
	return f.report, f.leaks, f.err
}

// fakeGitRunner：rev-parse 决定是否仓库；log/diff 返回预设的「新增路径」列表。
func fakeGitRunner(isRepo bool, addedOut string) gitx.Runner {
	return func(dir string, args ...string) (string, error) {
		switch args[0] {
		case "rev-parse":
			if isRepo {
				return "true\n", nil
			}
			return "false\n", nil
		case "log", "diff":
			return addedOut, nil
		}
		return "", fmt.Errorf("unexpected git %v", args)
	}
}

func runGitSecrets(t *testing.T, gl gitleaksFunc, run gitx.Runner, args ...string) (out, errOut string, code int) {
	t.Helper()
	var o, e bytes.Buffer
	code = 0
	deps := gitSecretsDeps{
		out:      &o,
		errOut:   &e,
		run:      run,
		gitleaks: gl,
		exit:     func(c int) { code = c },
		dir:      ".",
	}
	cmd := newGitSecretsCommand(deps)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	return o.String(), e.String(), code
}

const glOneFinding = `[{"RuleID":"aws-access-key","File":"config/app.go","StartLine":12,"Commit":"deadbeefcafe","Author":"Bob","Date":"2026-01-05T09:00:00Z","Secret":"REDACTED"}]`

func TestGitSecrets_CleanExitZero(t *testing.T) {
	gl := &fakeGitleaks{report: "[]"}
	_, errOut, code := runGitSecrets(t, gl.fn, fakeGitRunner(true, ""))
	if code != 0 {
		t.Errorf("干净应 exit 0，得到 %d", code)
	}
	if !strings.Contains(errOut, "未发现") {
		t.Errorf("应报「未发现」：%s", errOut)
	}
}

func TestGitSecrets_FindingsExitOne(t *testing.T) {
	gl := &fakeGitleaks{report: glOneFinding, leaks: true}
	out, _, code := runGitSecrets(t, gl.fn, fakeGitRunner(true, ""))
	if code != 1 {
		t.Errorf("有发现应 exit 1，得到 %d", code)
	}
	if !strings.Contains(out, "config/app.go:12") || !strings.Contains(out, "aws-access-key") {
		t.Errorf("应含命中细节：%s", out)
	}
	if !strings.Contains(out, "REDACTED") {
		t.Errorf("默认应脱敏：%s", out)
	}
}

func TestGitSecrets_DefaultRedacts(t *testing.T) {
	gl := &fakeGitleaks{report: "[]"}
	runGitSecrets(t, gl.fn, fakeGitRunner(true, ""))
	if !containsArg(gl.gotArgs, "--redact=100") {
		t.Errorf("默认应传 --redact=100，实际 args=%v", gl.gotArgs)
	}
}

func TestGitSecrets_ShowSecretsDisablesRedact(t *testing.T) {
	gl := &fakeGitleaks{report: "[]"}
	runGitSecrets(t, gl.fn, fakeGitRunner(true, ""), "--show-secrets")
	if !containsArg(gl.gotArgs, "--redact=0") {
		t.Errorf("--show-secrets 应传 --redact=0，实际 args=%v", gl.gotArgs)
	}
}

func TestGitSecrets_StagedFlag(t *testing.T) {
	gl := &fakeGitleaks{report: "[]"}
	runGitSecrets(t, gl.fn, fakeGitRunner(true, ""), "--staged")
	if !containsArg(gl.gotArgs, "--staged") {
		t.Errorf("--staged 应透传给 gitleaks，实际 args=%v", gl.gotArgs)
	}
}

func TestGitSecrets_LogOptsAndBaseline(t *testing.T) {
	gl := &fakeGitleaks{report: "[]"}
	runGitSecrets(t, gl.fn, fakeGitRunner(true, ""), "--log-opts=origin/main..HEAD", "--baseline=b.json")
	if !containsArg(gl.gotArgs, "--log-opts=origin/main..HEAD") {
		t.Errorf("--log-opts 应透传，实际 args=%v", gl.gotArgs)
	}
	if !containsArg(gl.gotArgs, "b.json") {
		t.Errorf("--baseline 应透传，实际 args=%v", gl.gotArgs)
	}
}

func TestGitSecrets_NotInstalledExitTwo(t *testing.T) {
	gl := &fakeGitleaks{err: errGitleaksNotInstalled}
	_, errOut, code := runGitSecrets(t, gl.fn, fakeGitRunner(true, ""))
	if code != 2 {
		t.Errorf("没装 gitleaks 应 exit 2，得到 %d", code)
	}
	if !strings.Contains(errOut, "brew install gitleaks") {
		t.Errorf("应给安装指引：%s", errOut)
	}
}

func TestGitSecrets_GitleaksRunErrorExitTwo(t *testing.T) {
	gl := &fakeGitleaks{err: fmt.Errorf("boom")}
	_, errOut, code := runGitSecrets(t, gl.fn, fakeGitRunner(true, ""))
	if code != 2 {
		t.Errorf("gitleaks 运行错误应 exit 2，得到 %d", code)
	}
	if !strings.Contains(errOut, "boom") {
		t.Errorf("应透出错误信息：%s", errOut)
	}
}

func TestGitSecrets_NotARepoExitTwo(t *testing.T) {
	gl := &fakeGitleaks{report: "[]"}
	_, errOut, code := runGitSecrets(t, gl.fn, fakeGitRunner(false, ""))
	if code != 2 {
		t.Errorf("非 git 仓库应 exit 2，得到 %d", code)
	}
	if !strings.Contains(errOut, "不是 git 仓库") {
		t.Errorf("应报非仓库：%s", errOut)
	}
}

func TestGitSecrets_FilenameAuditTriggersFinding(t *testing.T) {
	// gitleaks 干净，但历史里新增过 .env → 仍应 exit 1。
	gl := &fakeGitleaks{report: "[]"}
	out, _, code := runGitSecrets(t, gl.fn, fakeGitRunner(true, "config/.env\nREADME.md\n"))
	if code != 1 {
		t.Errorf("文件名命中应 exit 1，得到 %d", code)
	}
	if !strings.Contains(out, "config/.env") || !strings.Contains(out, "疑似敏感文件") {
		t.Errorf("应报可疑文件：%s", out)
	}
}

func TestGitSecrets_NoFilenamesSkipsAudit(t *testing.T) {
	gl := &fakeGitleaks{report: "[]"}
	_, _, code := runGitSecrets(t, gl.fn, fakeGitRunner(true, "config/.env\n"), "--no-filenames")
	if code != 0 {
		t.Errorf("--no-filenames 应跳过文件名审计 → exit 0，得到 %d", code)
	}
}

func TestGitSecrets_JSONOutput(t *testing.T) {
	gl := &fakeGitleaks{report: glOneFinding, leaks: true}
	out, _, code := runGitSecrets(t, gl.fn, fakeGitRunner(true, ""), "--json")
	if code != 1 {
		t.Errorf("有发现应 exit 1，得到 %d", code)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("--json 应合法：%s", out)
	}
	if v["detected"] != true {
		t.Errorf("detected 应为 true：%s", out)
	}
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}
