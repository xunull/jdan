# jdan

[简体中文](README.md) | **English**

A collection of handy little tools written in Go (single binary). The idea: each subcommand fixes one small pain point where a **built-in system tool behaves inconsistently / has ugly output / is missing on some platform**, bundled together so you don't have to install a dozen separate utilities. Design bias: smart by default (sensible defaults + auto-detection), but never take control away from the user (every automatic behavior can be overridden with a flag); text output is friendly by default, and `--json` is always there for scripts to consume.

## Installation

### Option 1: download a prebuilt binary (recommended)

Download the archive for your platform from [GitHub Releases](https://github.com/xunull/jdan/releases):

| Platform | Archive |
|------|---------|
| macOS Apple Silicon | `jdan_<VERSION>_darwin_arm64.tar.gz` |
| macOS Intel | `jdan_<VERSION>_darwin_amd64.tar.gz` |
| Linux x86_64 | `jdan_<VERSION>_linux_amd64.tar.gz` |
| Linux ARM64 | `jdan_<VERSION>_linux_arm64.tar.gz` |

```bash
# e.g. macOS Apple Silicon — replace <VERSION> with the version you downloaded
curl -L -o jdan.tar.gz https://github.com/xunull/jdan/releases/download/v<VERSION>/jdan_<VERSION>_darwin_arm64.tar.gz
tar xzf jdan.tar.gz
sudo mv jdan /usr/local/bin/
jdan version
```

Verify the SHA256 (`checksums.txt` is on the same Release page):

```bash
curl -LO https://github.com/xunull/jdan/releases/download/v<VERSION>/checksums.txt
shasum -a 256 -c checksums.txt --ignore-missing
```

### Option 2: go install

```bash
go install github.com/xunull/jdan@latest
```

With this method `jdan version` shows `dev / none / unknown`, because it doesn't go through goreleaser's ldflags injection.

### Option 3: build from source

```bash
git clone https://github.com/xunull/jdan.git
cd jdan
go build -o jdan .
# If a go.work in a parent directory breaks the build:
# Linux/macOS: GOWORK=off go build -o jdan .
# Windows PowerShell: $env:GOWORK="off"; go build -o jdan.exe .
```

## Shell completion

With this many commands, completion makes `<Tab>` pay off: it completes subcommand names, flag names, and for some flags the **values** too (`hash --algo <Tab>` → md5/sha1/sha256/sha512, `ascii-art --ramp <Tab>` → standard/detailed/blocks, `dns lookup --type`, `cal` months, `--doh` dynamic provider aliases, etc.). Powered by cobra; `jdan completion <shell>` generates the script:

```bash
# zsh (make sure compinit runs in ~/.zshrc)
jdan completion zsh > "${fpath[1]}/_jdan"   # then restart the shell or run compinit

# bash (needs bash-completion)
jdan completion bash | sudo tee /etc/bash_completion.d/jdan >/dev/null

# fish
jdan completion fish > ~/.config/fish/completions/jdan.fish

# powershell (add to $PROFILE)
jdan completion powershell | Out-String | Invoke-Expression
```

Try it in the current zsh session: `source <(jdan completion zsh)`.

## Commands

Index grouped by topic (the actual section order follows when each command was added, so the network and file categories aren't contiguous):

**Network & DNS**
- [`jdan http timing`](#jdan-http-timing) — measure the time spent in each phase of an HTTP request
- [`jdan http headers`](#jdan-http-headers) — show response headers + the full redirect chain (hop by hop)
- [`jdan http serve`](#jdan-http-serve) — temporary static file server + LAN URL + terminal QR code
- [`jdan net probe`](#jdan-net-probe) — client-side phase-by-phase probe (DNS/TCP/TLS/HTTP)
- [`jdan net selfcheck`](#jdan-net-selfcheck) — server-side self-check + external reachability prediction
- [`jdan ssl cert`](#jdan-ssl-cert) — inspect HTTPS certificate details (chain / verification / OCSP)
- [`jdan ssl scan`](#jdan-ssl-scan) — full TLS configuration audit (ssllabs-style A+/A/B/C/D/F grade)
- [`jdan ssl pin`](#jdan-ssl-pin) — generate the SPKI hash for cert pinning (6 formats)
- [`jdan cert`](#jdan-cert) — generate a self-signed TLS certificate for local development (make → inspect loop)
- [`jdan pem`](#jdan-pem) — inspect a PEM file offline (cert/CSR/private/public key; key↔cert match; never prints keys)
- [`jdan ssh-key`](#jdan-ssh-key) — SSH key parsing (info / fingerprint / pubkey, matching ssh-keygen)
- [`jdan dns lookup`](#jdan-dns-lookup) — query 6 record types concurrently, with DoH support
- [`jdan dns reverse`](#jdan-dns-reverse) — IP → hostname (PTR lookup)
- [`jdan dns trace`](#jdan-dns-trace) — iterative resolution from the root servers, showing the delegation path
- [`jdan ping`](#jdan-ping) — ping, with `--dns` to pick the DNS server that resolves the hostname
- [`jdan pubip4`](#jdan-pubip4--jdan-pubip6) / [`jdan pubip6`](#jdan-pubip4--jdan-pubip6) — look up your machine's public IP
- [`jdan ports`](#jdan-ports) — show the ports your machine is listening on (macOS)

**Files & Archives**
- [`jdan file bak`](#jdan-file-bak) — make a timestamped backup of a file
- [`jdan zip`](#jdan-zip) — pack a file or directory into a `.zip`
- [`jdan tree2`](#jdan-tree2) — show a two-level directory tree in multiple columns
- [`jdan disk`](#jdan-disk) — disk usage overview (per-mount capacity, df-style)
- [`jdan readme`](#jdan-readme) — print the README.md of a given directory (with bat highlighting)

**System**
- [`jdan macgpu`](#jdan-macgpu) — Apple Silicon GPU TUI monitor
- [`jdan unix-time`](#jdan-unix-time) — Unix timestamp → local time
- [`jdan cal`](#jdan-cal) — print a month/year calendar (highlights today, Monday start)

**Random Generation (CSPRNG)**
- [`jdan rand password`](#jdan-rand-password) — 1Password-style random password
- [`jdan rand uuid`](#jdan-rand-uuid) — UUID v4 / v7
- [`jdan uuid`](#jdan-uuid) — inspect a UUID (version/variant/v1·v7 timestamp/bytes)
- [`jdan rand hex`](#jdan-rand-hex--base64--base64url--base32) / [`base64`](#jdan-rand-hex--base64--base64url--base32) / [`base64url`](#jdan-rand-hex--base64--base64url--base32) / [`base32`](#jdan-rand-hex--base64--base64url--base32) — random bytes + encoding
- [`jdan rand alnum`](#jdan-rand-alnum) — alphanumeric string (no per-class constraint)
- [`jdan rand int`](#jdan-rand-int) — random integer in a closed interval
- [`jdan rand word`](#jdan-rand-word) — EFF diceware passphrase

**Test Data**
- [`jdan fake`](#jdan-fake) — generate realistic-looking fake values (name/email/uuid/date/ip…), reproducible with `--seed`

**Git**
- [`jdan git summary`](#jdan-git-summary) — repo at a glance (commits/branches/tags/age/contributors/hotspots)
- [`jdan git changelog`](#jdan-git-changelog) — generate a changelog from the latest tag to HEAD (grouped by Conventional Commits)

**Docs / Markdown**
- [`jdan toc`](#jdan-toc) — generate a table of contents from Markdown headings (GitHub-style anchors, can write back in place)

**Integrations**
- [`jdan obsidian install-claudian`](#jdan-obsidian-install-claudian) — install the Claudian Obsidian plugin

**Encoding & QR**
- [`jdan qr`](#jdan-qr) — generate a QR code (terminal / PNG / SVG)
- [`jdan qrwifi`](#jdan-qrwifi) — generate a WiFi join QR code (scan to connect, auto-escaped)
- [`jdan barcode`](#jdan-barcode) — generate a Code128 1D barcode (terminal / PNG / SVG)
- [`jdan figlet`](#jdan-figlet) — text → ASCII art banner (standard / block fonts)
- [`jdan morse`](#jdan-morse) — text ↔ Morse code (ITU, auto-detects direction)
- [`jdan img`](#jdan-img) — read an image file header and report dimensions/format/color/size (PNG/JPEG/GIF)
- [`jdan ascii-art`](#jdan-ascii-art) — render an image as ASCII art (optional truecolor)
- [`jdan mime`](#jdan-mime) — detect a file's real type by magic bytes (ignores the extension)
- [`jdan jwt decode`](#jdan-jwt-decode) — purely local JWT decoding (no signature verification, no network)
- [`jdan jwt verify`](#jdan-jwt-verify) — HMAC-verify a JWT signature (HS256/384/512, alg-confusion defense)
- [`jdan totp`](#jdan-totp) — TOTP 2FA codes (RFC 6238, compatible with Google Authenticator)
- [`jdan b64 enc/dec`](#jdan-b64) — base64 encode/decode (standard / URL-safe / no-pad)
- [`jdan url enc/dec`](#jdan-url) — URL percent-encoding
- [`jdan num`](#jdan-num) — base conversion (dec/hex/bin/oct) + bitwise operations
- [`jdan calc`](#jdan-calc) — arithmetic expression calculator (+ - * / % ^ + functions)
- [`jdan env`](#jdan-env) — .env file tools (lint / diff / redact / get)

**JSON / YAML / CSV**
- [`jdan json`](#jdan-json) — pretty/minify/path/keys/diff/lines/flatten/merge + yaml ↔ json + csv ↔ json

**Network / Lookup**
- [`jdan whois`](#jdan-whois) — domain/IP WHOIS (auto routing + IANA/ARIN referral following + parsed table)
- [`jdan ip`](#jdan-ip) — IP / CIDR calculation (info / contains / range / split / normalize)
- [`jdan meta`](#jdan-meta) — fetch page meta / Open Graph / Twitter Card (share-card audit)

**File Hashing & Archives**
- [`jdan hash`](#jdan-hash) — cross-platform md5/sha1/sha256/sha512 + `--check` verification
- [`jdan entropy`](#jdan-entropy) — compute Shannon entropy (tell if data is encrypted/compressed/random; sliding-window sparkline)
- [`jdan secrets-scan`](#jdan-secrets-scan) — scan for hardcoded secrets/tokens (regex + entropy; redacted output)
- [`jdan pwned`](#jdan-pwned) — check if a password is breached (HIBP k-anonymity, password never leaves your machine)
- [`jdan extract`](#jdan-extract) — general-purpose extraction for zip/tar/tar.gz/tar.bz2/gz/bz2



**Meta Commands**
- [`jdan version`](#jdan-version) — show version, commit, build time

### `jdan qr`

Generate a QR code from any string and print it to the terminal, or write it to a PNG / SVG file. **Use cases**: temporarily share a URL to your phone (scan it), embed a Wi-Fi password / config string in docs, or reuse it for the LAN URL that `jdan http serve` will print later.

**The terminal renders with half-height blocks** `▀▄` stacked together, halving the height so a long URL won't blow past 80 columns:

```bash
$ jdan qr "https://github.com/xunull/jdan"

█▀▀▀▀▀█ ▄█  ▀▀▄▄█▄▄█  █▀▀▀▀▀█
█ ███ █   ▄ ▄██▄▀ ▄▀  █ ███ █
█ ▀▀▀ █ ▀▀▄ ▄▄▀  ███▄ █ ▀▀▀ █
▀▀▀▀▀▀▀ ▀ ▀▄█▄█ █▄▀▄█ ▀▀▀▀▀▀▀
...
```

flags:

| flag | default | effect |
|------|------|------|
| `--ecc` | `M` | error-correction level `L/M/Q/H` (30%-correction H is good for codes with a logo or that may be partially obscured) |
| `--invert` | false | invert colors, for light-background terminals |
| `--full-block` | false | use full-width `██` instead of half-height blocks, for older terminals (e.g. some Windows CMD) |
| `--output <path>` | terminal | write a file based on the extension: `.png` / `.svg` |
| `--png-size <int>` | 256 | PNG pixel size |
| `--svg-module <int>` | 8 | SVG pixels per module |
| `--json` | false | output `{data, ecc, modules}` metadata |

stdin works too:

```bash
echo "data" | jdan qr
cat secret.txt | jdan qr --output secret.png --ecc H
```

Unsupported extensions (like `.jpg`) raise an error; for JPEG, convert from PNG yourself with `sips`/`ffmpeg`.

### `jdan qrwifi`

Generate a **WiFi join QR code**: point a phone camera at it and connect, no password to read aloud. A "typed payload" wrapper over `jdan qr` — it **escapes `\ ; , " :` in the SSID/password automatically** (the step that's easy to miss when hand-rolling a `WIFI:` string; miss it and the code is wrong and silently fails to join), and reuses the whole `qr` render pipeline, **zero new deps**.

Full technical doc: [docs/jdan-qrwifi.md](docs/jdan-qrwifi.md)

```bash
$ jdan qrwifi MyNetwork -p 's3cr3t'              # terminal QR
$ jdan qrwifi --ssid "Cafe Guest" --auth nopass  # open network, no password
$ jdan qrwifi Home -p pw --hidden                # hidden network
$ jdan qrwifi Home -p pw -o wifi.png             # save PNG (stick it on the wall)
$ jdan qrwifi Home --password-stdin <<< 'pw'     # password via stdin, off shell history
$ jdan qrwifi Home -p pw --json                  # {ssid,auth,hidden,payload,...}
```

The payload follows the `WIFI:T:<auth>;S:<ssid>;P:<password>;H:<hidden>;;` standard. Auth type `wpa` (default, covers WPA2/WPA3) / `wep` / `nopass` (open network, omits `P:`). **Forgetting the password on a WPA/WEP network is a hard error** (an empty-password code silently fails to join); for a genuinely open network pass `--auth nopass` explicitly. SSID via positional arg or `--ssid`. Password via `-p` (convenient) or `--password-stdin` (keeps it off shell history; the QR itself reveals the password by design, so this only guards the history/`ps` layer). Render flags (`--ecc`/`--invert`/`--full-block`/`--output .png/.svg`/`--json`) are all inherited from `qr`. **Deliberately out of scope**: enterprise 802.1X / EAP (complex payload, poor scanner support) and reading the system's saved WiFi password (would require digging into each platform's keychain).

### `jdan barcode`

Generate a **Code128 1D barcode** (common on inventory / logistics / shipping labels). **Zero new deps**: an embedded 107-row Code128 pattern table does the encoding, and rendering is hand-rolled (`image/png` is stdlib) — no external barcode library, the same "embedded table + algorithm" path as `lunar`.

Full technical doc: [docs/jdan-barcode.md](docs/jdan-barcode.md)

```bash
$ jdan barcode "ABC-123"           # terminal bars + human-readable text below
$ jdan barcode 5901234123457 -o label.png
$ jdan barcode "SKU42" -o tag.svg
$ echo "data" | jdan barcode       # stdin
$ jdan barcode "ABC-123" --json    # {data, code_set, checksum, modules}
```

**How it works**: a 1D barcode is a row of bars and spaces whose widths encode the data. Code128 has 107 symbols, each 11 modules wide (3 bars + 3 spaces), structured `[quiet] Start data checksum Stop [quiet]`, checksum = `(Start + Σ position×value) mod 103`. Code set defaults to **B** (printable ASCII 32-126); when the input is **all digits and even-length** it auto-switches to **C** (one symbol per 2 digits, half the width). Output: terminal (`█` bar columns + text) / PNG / SVG (`-o` by extension), with `--module` for thickness, `--invert`, `--no-text`. **Deliberately out of scope**: EAN-13 / UPC / Code39 (each has its own check-digit rules, can be added later), optimal mid-string code-set switching, and barcode *decoding* (image → digits needs image processing, same as `qr` not decoding). Note: PNG omits the human-readable text (would need a font dep); terminal and SVG render it.

### `jdan figlet`

Render text as an ASCII art banner (figlet style). Zero new dependencies (fonts are built in). It's part of the same "turn text into visual output" family as `jdan qr`.

Detailed technical docs: [docs/jdan-figlet.md](docs/jdan-figlet.md)

**Use cases**: add titles to CLI output, section separators, README banners, terminal MOTD, step prompts in scripts.

```bash
$ jdan figlet "jdan"
  ### ####   ###  #   #
   #  #   # #   # ##  #
   #  #   # ##### # # #
#  #  #   # #   # #  ##
 ##   ####  #   # #   #

# solid block font
$ jdan figlet "OK" --font block
 ███  █   █
█   █ █  █
█   █ ███
█   █ █  █
 ███  █   █

$ jdan figlet Deploy OK            # multiple args get joined
$ jdan figlet "Title" --center --width 60
$ echo "Build Done" | jdan figlet  # stdin
$ jdan figlet --list               # list fonts
```

Fonts: `standard` (outlined with `#`) / `block` (solid `█` blocks); covers A-Z / a-z / 0-9 / punctuation, lowercase folds to uppercase, no whitespace placeholder for unsupported characters. `--width` wraps automatically when a line is too long, `--center` centers it.

### `jdan morse`

Convert text ↔ International Morse code (ITU). **Auto-detects direction**: input containing only `.`/`-`/`/`/space → decode, otherwise encode. For learning / puzzles / fun. Zero new dependencies (one lookup table).

Detailed technical docs: [docs/jdan-morse.md](docs/jdan-morse.md)

```bash
$ jdan morse "SOS"
... --- ...

$ jdan morse "... --- ..."          # recognized as Morse → decode
SOS

$ jdan morse "Hello World"
.... . .-.. .-.. --- / .-- --- .-. .-.. -..

$ jdan morse "E" --encode            # force direction for short/ambiguous input (-d forces decode)
```

Letters separated by a single space, words by ` / `; case-insensitive, decode output is uppercase. Covers A–Z / 0–9 + standard punctuation. Unencodable characters (CJK/emoji) are skipped and unrecognized codes become `#`, with counts sent to stderr (stdout stays clean for pipes). `--json` gives `{direction, output}`.

### `jdan img`

Read an image's **file header** to report dimensions/format/color model/size, without decoding the whole image (`image.DecodeConfig`, constant-time even for huge images). Zero new dependencies (pure stdlib).

Detailed technical docs: [docs/jdan-img.md](docs/jdan-img.md)

**Supported**: PNG / JPEG / GIF (stdlib decoders; WEBP/BMP/TIFF need external dependencies, so they're out)

```bash
$ jdan img logo.png
logo.png
  格式: PNG
  尺寸: 512 x 512
  颜色: NRGBA (含 alpha)
  大小: 24.3 KiB

# multiple files → aligned table
$ jdan img hero.jpg thumb.jpg
hero.jpg   1920x1080  JPEG  340.0 KiB
thumb.jpg  320x180    JPEG   18.0 KiB

$ jdan img < logo.png         # stdin
$ jdan img *.png --json       # JSON array
```

When a file in a batch is corrupt/unsupported, it prints a one-line error and continues with the rest, then exits 1 overall (one bad file won't abort the whole batch). `--json` still outputs a valid empty array even if everything fails.

### `jdan ascii-art`

Render an image as **ASCII art** (like jp2a). Reuses the already-wired stdlib image decoders, **zero new dependencies**. The "draw it" counterpart to `img` (which only reads dimensions).

Detailed technical docs: [docs/jdan-ascii-art.md](docs/jdan-ascii-art.md)

```bash
$ jdan ascii-art logo.png            # auto-scale to terminal width
$ jdan ascii-art photo.jpg -w 60     # explicit column width
$ jdan ascii-art logo.png --color    # 24-bit truecolor (TTY only)
$ jdan ascii-art logo.png --invert   # invert (light-background terminals)
$ cat x.png | jdan ascii-art         # stdin
```

Algorithm: decode → slice into a grid by column width with **box-average** sampling per cell → map luminance to a character ramp, optionally coloring each char with truecolor. Ramp (dark→light): `standard` (default 10-level, width-1 safe) / `detailed` (70-level) / `blocks` (`░▒▓█`, but these are East-Asian-ambiguous and render 2-wide in CJK terminals — it warns) / a custom string. Default is **monochrome** (pasteable into READMEs/comments, pipeable); `--color` adds truecolor on a TTY (stripped when piped). Character aspect is corrected by 0.5 by default (terminal cells are ~2x tall) to avoid vertical stretch. Formats: PNG/JPEG/GIF (first frame); WebP/HEIC unsupported (would need a new dependency).

### `jdan mime`

Detect a file's real MIME / content-type by its **content** (magic bytes), **ignoring the extension** — it recognizes the type even if the file was renamed. Zero new dependencies (pure stdlib).

Detailed technical docs: [docs/jdan-mime.md](docs/jdan-mime.md)

The base is stdlib `http.DetectContentType` (~60 types), plus a curated magic table that fills in dev formats stdlib misses (ELF / 7z / xz / zstd / bzip2 / tar / SQLite).

```bash
$ jdan mime logo.png
image/png

# multiple files → aligned table
$ jdan mime a.bin b.bin c.bin
a.bin   application/pdf
b.bin   application/zip
c.bin   text/plain; charset=utf-8

# recognizes a renamed file, and flags the extension mismatch
$ mv photo.png weird.txt
$ jdan mime weird.txt
image/png   (扩展名 .txt 不符)

$ jdan mime < file.bin        # stdin
$ jdan mime *.bin --json      # JSON array
```

Extension-mismatch detection uses a built-in extension→type table (OS-independent, reproducible) and deliberately does not fall back to the stdlib lookup that depends on the system's mime.types. Empty file → `inode/x-empty`. A bad file in a batch won't abort the rest; it exits 1 overall. Complements `jdan img` (which focuses on image dimensions).

### `jdan meta`

Fetch a page's `<meta>` / **Open Graph** / **Twitter Card** / canonical / favicon — answers "what will this link look like when shared on WeChat/Twitter/Slack" plus a share/SEO **audit**. Reuses `x/net/html` (already in the dep graph) + the existing HTTP stack, **zero new deps**.

Full technical doc: [docs/jdan-meta.md](docs/jdan-meta.md)

```bash
$ jdan meta https://example.com/article
$ jdan meta example.com --json
$ cat page.html | jdan meta          # parse local HTML offline
$ jdan meta page.html                # parse a local file
```

Fetch constraints: follows redirects and reports the final URL; rejects non-`text/html`; reads only the `<head>` region (capped at 512 KiB, no whole-page download); 10s timeout by default (`--timeout`). Sends a common browser User-Agent by default (many sites serve a stripped page to non-browser UAs); `--ua` lets you impersonate a specific platform crawler. The audit flags missing key tags like `og:image`/`og:title`/`description`/`canonical`. **Reads static HTML only, no JS**: SPAs that inject tags via JS won't be picked up (reflected honestly, not a bug). Parsing uses the proper `x/net/html` tokenizer, so malformed HTML is handled robustly.

### `jdan jwt decode`

Decode a JWT's header and payload purely locally — **no signature verification, no network requests whatsoever**. For everyday debugging you usually don't need verification: you just want a quick look at what claims the token actually carries, and you don't want to paste a token that may contain PII into an online tool like jwt.io.

```
$ jdan jwt decode "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImFiYyJ9..."

Header:
  {
    "alg": "RS256",
    "kid": "abc",
    "typ": "JWT"
  }

Payload:
  {
    "iat": 1516239022,
    "name": "John Doe",
    "sub": "1234567890"
  }

算法: RS256
Key ID: abc
Subject: 1234567890
签发: 2018-01-18 01:30:22 UTC
Signature: (present, 21 chars base64url)
```

**Design details**:

- **No JWT library**: a JWT's three base64url segments can be cracked open with 20 lines of stdlib; pulling in `golang-jwt` would instead expose the secret/key API surface and mislead users into thinking this tool does signature verification
- **The signature segment shows only a character count** in text output, never the raw value, to avoid accidentally pasting it into a PR / log / Slack
- **`--json` output includes the full signature** (script scenarios need it for a verify pipeline)
- exp / iat / nbf are automatically interpreted as RFC 7519 NumericDate (unix seconds); an expired token is marked "expired", a valid one shows the remaining time (compact form like `3d 4h`)
- `aud` supports both `string` and `[]string`, the two RFC 7519-legal forms

flags:

| flag | effect |
|------|------|
| `--header-only` | only output the header (don't print the payload; good when you just want to see alg/kid) |
| `--json` | structured JSON output including the full signature, for script consumption |
| `--raw` | don't pretty-print, output compact JSON |

stdin input works too (good for piping down from commands like `kubectl get secret`):

```bash
echo "$TOKEN" | jdan jwt decode
kubectl get secret my-jwt -o jsonpath='{.data.token}' | base64 -d | jdan jwt decode
```

**Features not provided** (design trade-offs):

- `decode` does no signature verification — verification is the separate `jdan jwt verify` subcommand (below)
- No fetching the issuer's jwks_uri — any network behavior belongs to `verify`, not `decode`
- No JWT construction — same as above

### `jdan jwt verify`

Verify a JWT signature with an HMAC secret (HS256/384/512). `decode` only inspects the contents and never verifies; `verify` does just the verification and needs `--secret`.

Detailed technical docs: [docs/jdan-jwt-verify.md](docs/jdan-jwt-verify.md)

```bash
$ jdan jwt verify "$TOKEN" --secret mykey
✓ 签名有效 (HS256)            # valid → exit 0

$ jdan jwt verify "$TOKEN" --secret wrong
✗ 签名无效 (HS256)            # invalid → exit 1 (easy to gate in scripts)

$ echo "Bearer $TOKEN" | jdan jwt verify --secret mykey   # stdin + auto-strips Bearer
$ jdan jwt verify "$TOKEN" --secret mykey --json          # {alg, valid}
```

**Security — alg-confusion defense**: verification goes by `header.alg`. If the token is `RS256`/`ES256` and you pass `--secret`, it errors out and refuses, never treating a non-HMAC token as HMAC (this is exactly the classic alg-confusion attack surface). HMAC comparison uses `crypto/hmac.Equal` (constant time).

flags:

| flag | purpose |
|------|---------|
| `--secret` | HMAC secret (verifies only when given, HS256/384/512 only) |
| `--json` | structured output `{alg, valid}` |

**Intentionally not done**: RS*/ES* public-key verification (needs reading a PEM public key + each curve, a later `--key` could add it); issuing JWTs (jdan leans inspector).

### `jdan hash`

Compute a file's md5 / sha1 / sha256 / sha512 across platforms. **Streaming** (doesn't read everything into memory, 1GB+ files are fine); with multiple algorithms it reads once and computes in parallel (`io.MultiWriter` feeds multiple hashers).

**Why a dedicated command**: macOS's `shasum -a 256` doesn't have the same command name as Linux's `sha256sum`; `md5sum` doesn't exist on macOS at all (it's called `md5`); and the output format differs slightly too. `jdan hash` is consistent across platforms + its output format is compatible with the system tools.

```bash
$ jdan hash file.zip
edeaaff3f1774ad2888673770c6d64097e391bc362d7d6fb34982ddf0efd18cb  file.zip

$ jdan hash file.zip --algo md5,sha256
MD5:    0bee89b07a248e27c83fc3d5951213c1
SHA256: edeaaff3f1774ad2888673770c6d64097e391bc362d7d6fb34982ddf0efd18cb
file:   file.zip

$ jdan hash file.zip --all
MD5:    0bee89b07a248e27c83fc3d5951213c1
SHA1:   03cfd743661f07975fa2f1220c5194cbaff48451
SHA256: edeaaff3...
SHA512: 4f285d0c...

$ echo "hi" | jdan hash -
98ea6e4f216f2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4  -
```

**`--check` mode** (output is byte-equal to `shasum -c` / `sha256sum -c`):

```bash
$ cat checksums.txt
abc123...sha256...  file1.zip
def456...sha256...  file2.tar

$ jdan hash --check checksums.txt
file1.zip: OK
file2.tar: OK

2 total, 0 failed
```

If anything is FAILED → exit 1, handy for monitoring / CI gates. **The algorithm is auto-detected from the hash length**: 32 chars = md5, 40 = sha1, 64 = sha256, 128 = sha512. So `--check` doesn't need an extra `--algo` flag.

**flags**:

| flag | default | effect |
|------|------|------|
| `--algo` | `sha256` | csv: `md5,sha256` runs multiple algorithms in one read |
| `--all` | false | run md5 + sha1 + sha256 + sha512 (overrides `--algo`) |
| `--check <file>` | none | verification mode; FAILED → exit 1 |
| `--json` | false | structured output |

**Deliberately out of scope**:

- xxh3 (a non-cryptographic but 4 GB/s hash) — pulls in a third-party dep (`github.com/zeebo/xxh3`), will add it when someone actually needs it
- BLAKE2 / BLAKE3 — same as above
- `--binary` flag (matching GNU `sha256sum -b`) — text / binary modes are no different on Unix

### `jdan entropy`

Compute the **Shannon entropy** of a string / file / stdin (information content of the byte distribution, 0–8 bits/byte). High (≥7.5) ≈ encrypted/compressed/random, low ≈ repetitive/structured text. Use it to tell whether data is encrypted/compressed, spot high-entropy secrets in a file, or gauge compressibility. Zero new dependencies (pure `math`).

Detailed technical docs: [docs/jdan-entropy.md](docs/jdan-entropy.md)

```bash
$ jdan entropy "hello world"
bytes:    11
entropy:  2.85 bits/byte   (低：文本/结构化)
total:    31.3 bits
distinct: 8 / 256 字节值

$ head -c 4096 /dev/urandom | jdan entropy | head -2
bytes:    4096
entropy:  7.96 bits/byte   (极高：疑似加密/压缩/随机)

# sliding-window sparkline: spot the compressed/encrypted region in a firmware/binary blob at a glance
$ jdan entropy -f firmware.bin --window 512
▁▁▂▃█████▇▆▂▁
峰值 7.97 @ 偏移 0x1A00

# --charset: an extra "search-space bits" estimate (explicitly NOT a strength score)
$ jdan entropy "Tr0ub4dour" --charset
charset:  62 符号集 ≈ 59.5 bits（搜索空间，非强度评分）
```

Input: positional arg = string / `-f` = file / no arg = stdin; `--json` for structured output. **"Entropy" is anchored to the strict Shannon definition** (data randomness); it does not pretend to be a password strength score — real strength needs dictionary/pattern checks (the zxcvbn approach, which needs a library).

### `jdan secrets-scan`

Scan files / directories / stdin for **hardcoded secrets / tokens**: a regex engine (known formats, high precision) + an entropy engine (unknown tokens, reusing `entropy`). A lean gitleaks/trufflehog. Zero new dependencies.

Detailed technical docs: [docs/jdan-secrets-scan.md](docs/jdan-secrets-scan.md)

```bash
$ jdan secrets-scan .
config/prod.env:7   [aws-access-key]  AKIA…J7QF  (high)
src/client.go:42    [generic-assign]  Xy9K…P6dC  (medium)
deploy.sh:3         [high-entropy]    dGhp…YWVo  (low, entropy 4.6)

共 3 处疑似密钥（已脱敏；exit 1）
```

**Security rule: the output never contains the full secret** — only a redacted preview (first 4…last 4). A scanner must not become a leak itself (same principle as `jdan pem`, pinned by a security test). Noise control: an embedded allowlist (UUID / example placeholders), inline `# pragma: allowlist secret` exemption, a tunable `--min-entropy`, and low confidence on entropy hits. `.git`/`node_modules`/binary/lock files are skipped by default (`-a` scans everything). Exit codes: 0 nothing found / 1 found (CI gate) / 2 error. `--json` is also secret-free. Git-history scanning is intentionally out of scope (v1).

### `jdan pwned`

Check whether a password appears in known data breaches (via Have I Been Pwned's Pwned Passwords), **without the password ever leaving your machine**. Reuses stdlib `crypto/sha1` + `net/http` + `x/term`, **zero new deps**.

Full technical doc: [docs/jdan-pwned.md](docs/jdan-pwned.md)

**How it works (k-anonymity)**: compute `SHA1(password)` locally, send only the **first 5 hex chars** to `api.pwnedpasswords.com/range/<prefix>`; the server returns all hash suffixes sharing that prefix plus their breach counts, and you compare the remaining 35 chars locally. The server only ever sees a 5-char prefix (which matches hundreds of thousands of possible passwords); your plaintext and full hash never go over the wire.

```bash
$ jdan pwned                       # no-echo prompt (hidden, off shell history)
$ echo -n 'password' | jdan pwned
⚠ this password has appeared in known breaches 52,372,427 times — stop using it

$ cat passwords.txt | jdan pwned --batch   # line-by-line audit
$ echo -n 'pw' | jdan pwned --json
```

Input comes only from a **no-echo prompt** or stdin (there is **deliberately no `-p` flag** — a leak-checking tool shouldn't leave your password in shell history). `Add-Padding: true` is on by default (fixed-length response, so even the size leaks nothing). Exit codes: 0 clean / 1 pwned / 2 error — usable as a CI / pre-commit gate. **Deliberately out of scope**: looking up breached accounts by email (that API needs a paid key and sends your real email, with no k-anonymity protection).

### `jdan extract`

General-purpose extraction. Recognizes 8 formats (by file extension), and rejects directory traversal (`..` escaping the root).

**Why a dedicated command**: `tar xzvf` vs `unzip` vs `bzip2 -d` each have different syntax, and picking the wrong command errors out. `jdan extract <anything>` auto-detects the format by extension.

```bash
$ jdan extract release.tar.gz
✓ extracted 42 entry(ies) to release

$ jdan extract data.zip -o /tmp/out
✓ extracted 7 entry(ies) to /tmp/out

$ jdan extract docs.zip --here          # no subdirectory, extract to cwd
✓ extracted 12 entry(ies) to .

$ jdan extract data.zip --list          # just list contents, don't extract
archive: data.zip  (5 entries, 1.2MB total)

           1.2KB  README.md
           300KB  bin/foo
  d            -  bin/
           950KB  data.json
```

**Default behavior**: extract into a `<archive-name>/` subdirectory of the current directory. For `.tar.gz` / `.tar.bz2` / `.tgz` the **double suffix** is stripped (`release.tar.gz` → `release/`).

**Supported formats**:

| Format | Detected suffix |
|------|---------|
| zip | `.zip` |
| tar | `.tar` |
| tar.gz | `.tar.gz` / `.tgz` |
| tar.bz2 | `.tar.bz2` / `.tbz2` / `.tbz` |
| gz (single file) | `.gz` (output drops the `.gz` suffix) |
| bz2 (single file) | `.bz2` |

**flags**:

| flag | default | effect |
|------|------|------|
| `-o` / `--output` | `<archive-name>/` subdirectory | extraction target directory |
| `--here` | false | extract to cwd (no subdirectory) |
| `--list` | false | just list contents, don't actually extract |
| `--json` | false | structured output |

**Security**:

- **Reject directory traversal**: an entry name containing a `..` segment is rejected outright (not silently sanitized) — the standard defense against zip slip attacks
- **Reject absolute-path entries**: names like `/etc/passwd` are also rejected
- **Reject symlink entries**: symlinks in tar are skipped (prevents symlink-then-write attacks)
- **4 GiB per-entry cap**: prevents zip bombs

**Deliberately out of scope**:

- `.7z` — external lib (`github.com/saracen/go7z` or shelling out to the 7zz binary) is complex
- `.tar.xz` — Go stdlib has no lzma; pulling in `github.com/ulikunitz/xz` is a new dep, will add it when someone actually needs it
- `.rar` — patent issues

### `jdan totp`

A TOTP 2FA code tool (RFC 6238). Three subcommands cover **generate / parse otpauth URI / verify**. Compatible with Google Authenticator / Authy / 1Password.

Detailed technical docs: [docs/jdan-totp.md](docs/jdan-totp.md)

**Why a dedicated command**: when the secret is already on your machine (dotfiles / password-manager export / CI secret), the CLI generates the code directly and beats the "grab phone → open app → find entry → read out digits" routine; and in scripts it can auto-fill 2FA. Zero new dependencies (`crypto/hmac` + `encoding/base32` are both stdlib).

> ⚠️ **The secret is a long-lived credential.** Passing it as an arg lands it in shell history + the process list (`ps`), so that's only fine for temporary/testing use. For long-term use, go through stdin or an environment variable.

```bash
# generate the current code (defaults match Google Authenticator: SHA1/6 digits/30s)
$ jdan totp code JBSWY3DPEHPK3PXP
283461   (expires in 17s)

# safer ways to supply the secret (don't enter history)
$ echo "$SECRET" | jdan totp code -
$ JDAN_TOTP_SECRET="$SECRET" jdan totp code

# parse an otpauth URI from a scanned QR code (its parameters are used automatically)
$ jdan totp uri "otpauth://totp/GitHub:quincy?secret=JBSWY3DP&issuer=GitHub&digits=6&period=30"
Issuer:    GitHub
Account:   quincy
Algorithm: SHA1
Digits:    6
Period:    30s
Code:      283461   (expires in 17s)

# verify a code (exit code 0/1, --window tolerates clock drift)
$ jdan totp verify JBSWY3DPEHPK3PXP 283461
✓ valid

# JSON for script consumption
$ jdan totp code JBSWY3DPEHPK3PXP --json
{"code":"283461","expires_in":17,"period":30,"digits":6}
```

**base32 secret tolerance**: lowercase / space-grouped (Google's display format) / missing padding are all handled automatically. For the few services that use SHA256 or 8-digit codes, override with `--algo` / `--digits`, or just use `uri` (the parameters are in the URI).

The implementation is **byte-equal to the official RFC 6238 / RFC 4226 test vectors** (the gold standard for a TOTP implementation).

### `jdan b64`

base64 encode/decode. Supports the standard / URL-safe alphabets + optional padding.

```bash
$ jdan b64 enc "hello world"
aGVsbG8gd29ybGQ=

$ jdan b64 dec "aGVsbG8gd29ybGQ="
hello world

$ jdan b64 enc "data" --url --no-pad      # URL-safe + strip padding
ZGF0YQ

$ echo "secret" | jdan b64 enc -          # stdin
c2VjcmV0Cg==

$ jdan b64 enc -i input.bin -o out.b64    # file → file
```

| flag | effect |
|------|------|
| `--url` | URL-safe alphabet (`-_` replaces `+/`) |
| `--no-pad` | strip the trailing `=` padding (for enc) |
| `-i <file>` | read from a file |
| `-o <file>` | write to a file |
| `--no-newline` | don't append a trailing newline to enc output (for script pipes) |

**dec auto-detects padding**: with `=` it uses standard, without it uses raw. No flag needed.

### `jdan url`

URL percent-encoding / decoding (RFC 3986).

```bash
$ jdan url enc "hello world"
hello%20world

$ jdan url dec "hello%20world"
hello world

$ jdan url enc "a b" --query              # query-string mode (+ for space)
a+b

$ jdan url dec "a+b" --query
a b
```

**path vs query mode**:

| Mode | Space encoded as | Use case |
|------|-----------|------|
| default / `--path` | `%20` | URL path segments / most scenarios |
| `--query` | `+` | URL query strings (compatible with application/x-www-form-urlencoded) |

### `jdan num`

A base-conversion + bitwise-operation tool. The main command auto-detects the input base and outputs dec/hex/bin/oct + bit info all at once; the `bit` subcommand does bitwise operations. uint64 range, zero new dependencies (pure `strconv` + `math/bits`).

Detailed technical docs: [docs/jdan-num.md](docs/jdan-num.md)

**Why a dedicated command**: inspecting a register value, Unix permission bits, a flag mask, or a subnet mask means converting between bases, and reaching for a calculator / firing up python is slow. `jdan num` prints every base + a bit display in one line, and the `bit` subcommand computes bitwise operations directly.

```bash
# auto-detect the base (0x/0b/0o/leading-0/decimal), output everything at once
$ jdan num 0xDEADBEEF
Decimal:  3735928559
Hex:      0xDEADBEEF
Binary:   0b11011110101011011011111011101111
Octal:    0o33653337357
Bits:     24 set (...), width 32

# bit display (for inspecting flags / masks)
$ jdan num 0b10110 --bits
...
          bit:  4 3 2 1 0
          val:  1 0 1 1 0

# binary zero-padded to align with a register
$ jdan num 0xFF --width 16
Binary:   0b0000000011111111

# bitwise operations (AND/OR/XOR/NOT/<</>>, with symbol aliases & | ^ ~)
$ jdan num bit "0xFF AND 0x0F"
0x0F  (15, 0b1111)
$ jdan num bit "1 << 8"
0x100  (256, 0b100000000)
$ jdan num bit "NOT 0xFF" --width 8
0x0  (0, 0b0)

# JSON for script consumption
$ jdan num 255 --json
{"decimal":255,"hex":"0xFF","binary":"0b11111111","octal":"0o377","bits_set":8,"bit_width":8}
```

**uint64 range**, with negatives / over-64-bit values reported as a clear error rather than silently wrapping. It belongs to the same "encoding/base" family as `jdan hash` / `jdan b64`.

### `jdan calc`

A command-line arithmetic expression calculator. A hand-written recursive-descent parser, supporting the four basic operations / exponentiation / modulo / parentheses / unary minus / functions / constants / base-prefixed operands. Zero new dependencies (pure `math` + `strconv`).

Detailed technical docs: [docs/jdan-calc.md](docs/jdan-calc.md)

**Why a dedicated command**: quick math on the command line is awkward — `python3 -c` is slow to start, `bc` has cryptic errors and no `^` exponent, and shell `$((...))` is integer-only with no functions. `jdan calc` does it in one line.

```bash
$ jdan calc "3 * (4 + 5) / 2"     # 13.5
$ jdan calc "2 ^ 10"              # 1024 (^ is right-associative, also accepts **)
$ jdan calc "-5 + 3"              # -2 (an expression can start with a minus sign)
$ jdan calc "sqrt(2)"            # 1.4142135623730951
$ jdan calc "max(3, 7, 2)"       # 7
$ jdan calc "pi * 2"             # 6.283185307179586
$ jdan calc "0xFF + 1"           # 256 (base-prefixed operand)
$ jdan calc "255 + 1" --hex      # 0x100
$ jdan calc "10 / 3" --precision 2  # 3.33
$ echo "1 + 2 * 3" | jdan calc   # stdin → 7
```

**Functions**: sqrt/abs/floor/ceil/round/ln/log10/sin/cos/tan/min/max; **constants**: pi/e/tau (case-insensitive, can nest). Errors carry position info, friendlier than bc.

**Boundary**: arithmetic + functions belong to `jdan calc`; bitwise operations (AND/OR/XOR/shift) belong to `jdan num bit`, with no overlap.

### `jdan env`

A `.env` file inspection tool. Four subcommands cover **lint / diff / redact / get**. It leans toward "inspect / compare / redact" and doesn't do loading (that's `direnv` / `dotenv-cli`). Zero new dependencies.

Detailed technical docs: [docs/jdan-env.md](docs/jdan-env.md)

**Why a dedicated command**: `.env` problems are sneaky — a missing key in prod surfaces only at deploy time, an unquoted value with spaces gets truncated by the shell, a duplicate key quietly overrides another, and pasting one into an issue leaks a secret. `jdan env` turns each of these into a one-line command.

```bash
# lint: 6 categories of checks, error → exit code 1 (CI gate)
$ jdan env lint .env
.env:3   warning  duplicate key DATABASE_URL (first at line 1)
.env:5   warning  unquoted value with spaces: KEY=hello world
.env:6   error    invalid key name "2FOO" (must match [A-Za-z_][A-Za-z0-9_]*)

# diff: catch missing keys before deploy (compares keys only by default, no value leak)
$ jdan env diff .env.example .env
Only in .env.example (3):
  + STRIPE_SECRET_KEY
  + REDIS_URL
  + SMTP_HOST
Common keys: 12
$ jdan env diff .env.example .env.prod --exit-code && echo "all keys present"

# redact: safely paste into an issue after masking
$ jdan env redact .env | pbcopy
DATABASE_URL=po**************************db
export API_KEY=sk***********56

# get: more reliable than grep+cut (handles quotes / export / inline comments)
$ jdan env get .env DATABASE_URL
postgres://localhost:5432/mydb
```

Supports quotes / the `export` prefix / inline comments / last-wins for duplicate keys (shell semantics). `--strict` (also blocks on warnings) / `--values` (diff compares values) / `--full` / `--keep-short` (redact strategies).

### `jdan json`

A JSON toolkit (**13 subcommands**). Design goal: zero learning curve for common operations, **not a jq replacement**. Use jq for complex queries; jdan json covers the 80% of everyday high-frequency cases.

Detailed technical docs: [docs/jdan-json.md](docs/jdan-json.md)

**Why a dedicated command**: `python -m json.tool` prettifies but its arguments are hard to remember + it drops number precision; `jq` is powerful but its syntax is steep; converting YAML / CSV to JSON means separately installing `yq` / `csvjson`; and JSONL (structured logs) has no handy command. `jdan json` handles all of it with one set of commands.

```bash
# pretty / minify (preserves number precision, 2^53 + 1 isn't lost)
$ jdan json pretty data.json
$ jdan json minify data.json --in-place

# get a value by path (dot-path / bracket / RFC 6901, pick one, can mix)
$ jdan json path "users[0].name" data.json
"alice"
$ jdan json path "users.0.name" data.json -r       # -r strips quotes
alice
$ jdan json path "/users/0/name" data.json --pointer

# list keys (top-level / recursively all paths)
$ jdan json keys data.json --all
age
name
users[0].email
users[0].name

# semantic diff (outputs an RFC 6902 JSON Patch)
$ jdan json diff a.json b.json
~ /age: 30 -> 31
+ /new = true
$ jdan json diff a.json b.json --json              # RFC 6902 patch
$ jdan json diff schema.json prod.json --exit-code # CI gate

# JSONL (structured logs, one JSON per line)
$ jdan json lines --count < logs.jsonl
12847
$ jdan json lines --head 5 < logs.jsonl

# YAML ↔ JSON (numbers, nesting, big ints all keep precision)
$ jdan json from-yaml config.yaml > config.json
$ jdan json to-yaml config.json > config.yaml

# CSV ↔ JSON (UTF-8 BOM auto-stripped, quoted fields handled correctly)
$ jdan json from-csv users.csv               # → array of objects
$ jdan json from-csv data.tsv --delim '\t'
$ jdan json to-csv users.json --header "name,age"

# flatten ↔ unflatten (nested ↔ dotted keys; key format = json path expressions)
$ echo '{"a":{"b":1,"c":[10,20]}}' | jdan json flatten
{"a.b":1,"a.c[0]":10,"a.c[1]":20}
$ echo '{"a.b":1,"a.c":2}' | jdan json unflatten
{"a":{"b":1,"c":2}}

# merge (deep merge, later overrides earlier; objects merge recursively, not wholesale replace)
$ jdan json merge defaults.json prod.json local.json     # config layering
$ jdan json merge a.json b.json --arrays append          # concatenate arrays instead of replacing
```

`flatten`/`unflatten`: see [docs/jdan-json-flatten.md](docs/jdan-json-flatten.md) — round-trips (empty containers preserved + big-int precision), `--sep` to change the separator, sparse arrays filled with null, object/array conflict detection. `merge`: see [docs/jdan-json-merge.md](docs/jdan-json-merge.md) — objects merge recursively, `--arrays replace/append`, `-`=stdin, big-int precision preserved, inputs not mutated.

**Working with jq**:

```bash
# convert YAML → JSON then query with jq
$ jdan json from-yaml config.yaml | jq '.servers[].port'

# convert CSV → JSON then grab the name field of the first row
$ jdan json from-csv users.csv --pretty=false | jdan json path "0.name" -r
```

### `jdan whois`

A WHOIS lookup command (RFC 3912). Auto-detects domain vs IP, auto-routes to the correct server, follows IANA / ARIN referrals to the final response, and **outputs a parsed field table by default**.

Detailed technical docs: [docs/jdan-whois.md](docs/jdan-whois.md)

**Why a dedicated command**: macOS's built-in BSD `whois` has an outdated TLD mapping table (many new gTLDs aren't recognized); Linux needs `apt install whois`; Windows has no native support; and on every platform the raw text output relies on you grepping it by eye. `jdan whois` is cross-platform with zero config + 53 built-in TLD mappings + an IANA fallback + a parsed table, so the key fields (expiry/registrar/nameservers) are visible at a glance.

```bash
$ jdan whois example.com
Target:    example.com (domain)
Server:    whois.verisign-grs.com

  Domain:         EXAMPLE.COM
  Registrar:      RESERVED-Internet Assigned Numbers Authority
  Created:        1995-08-14 04:00 UTC  (31 years ago)
  Expires:        2026-08-13 04:00 UTC  (in 2 months)
  DNSSEC:         signedDelegation
  Status:         clientDeleteProhibited
                  clientTransferProhibited
                  clientUpdateProhibited
  Nameservers:    elliott.ns.cloudflare.com
                  hera.ns.cloudflare.com

$ jdan whois 193.0.0.1                        # IPv4 → ARIN → followed to RIPE
Target:    193.0.0.1 (ipv4)
Server:    whois.ripe.net
Chain:     whois.arin.net -> whois.ripe.net

  Range:          193.0.0.0 - 193.0.7.255
  Org:            Reseaux IP Europeens Network Coordination Centre (RIPE NCC)
  Country:        NL
  Abuse email:    abuse@ripe.net

$ jdan whois example.com --raw                # raw WHOIS text
$ jdan whois example.com --full               # parsed table + raw text
$ jdan whois example.com --json               # structured JSON (includes parsed)
$ jdan whois example.com --server custom.whois.com  # override the default server
```

**Pairs with jdan ssl cert**: cert shows TLS expiry, whois shows domain registration expiry — both need monitoring:

```bash
# monitoring pipeline example
jdan whois example.com --json | jdan json path "parsed.expiry_date" -r
# → 2026-08-13T04:00:00Z

jdan ssl cert example.com --json | jdan json path "not_after" -r
# → 2026-XX-XX (cert expiry)
```

**Parser fallback**: if the parser fails (an unrecognized schema like `.br`) → it **automatically falls back to raw**, so there's always content; `--raw` always gives you the original text and is a 1st-class citizen.

### `jdan ip`

An IP address & CIDR calculation toolkit. Five subcommands cover **combined info / subnet membership check / IP list / subnetting / IPv6 normalization**.

Detailed technical docs: [docs/jdan-ip.md](docs/jdan-ip.md)

**Why a dedicated command**: the daily toolchain for SREs / network admins / backend devs is fragmented (online CIDR calculators, `ipcalc` isn't cross-platform, `sipcalc` isn't installed by default on macOS), and there's no single interface that takes both IP and CIDR, IPv4 and IPv6. `jdan ip` handles all of it with one set of commands, zero new dependencies (pure Go stdlib `net/netip`).

```bash
# combined info (takes IP / CIDR / IPv4 / IPv6)
$ jdan ip info 192.168.1.0/24
  CIDR:           192.168.1.0/24
  Version:        IPv4
  Network:        192.168.1.0
  Broadcast:      192.168.1.255
  First host:     192.168.1.1
  Last host:      192.168.1.254
  Netmask:        255.255.255.0
  Wildcard:       0.0.0.255
  Total IPs:      256
  Usable:         254

$ jdan ip info 192.168.1.42                  # single IP: classification + binary/hex/decimal + reverse-DNS
  Address:        192.168.1.42
  Version:        IPv4
  Hex:            0xC0A8012A
  Decimal:        3232235818
  Binary:         11000000.10101000.00000001.00101010
  Reverse DNS:    42.1.168.192.in-addr.arpa
  Private:        yes

# exit code: CI-gate friendly
$ jdan ip contains 10.0.0.0/8 10.5.1.2 && echo "internal"
internal
$ jdan ip contains 10.0.0.0/8 10.5.1.2 --verbose
yes

# subnetting
$ jdan ip split 10.0.0.0/22 24
10.0.0.0/24
10.0.1.0/24
10.0.2.0/24
10.0.3.0/24
(4 subnets)

# list IPs (16 by default, --limit 0 lists all, hard cap of 1M prevents OOM)
$ jdan ip range 192.168.1.0/29
192.168.1.0
...
192.168.1.7
(8 total)

# IPv6 expand / compact
$ jdan ip normalize 2001:db8::1 --expand
2001:0db8:0000:0000:0000:0000:0000:0001
```

**Classification fields** cover RFC 1918 / 3849 / 4193 / 5737 / 6598: Private / Loopback / Multicast / Link-local / Doc range / Unique local / CGNAT / Global unicast all get tagged.

**Pairs with whois / dns**:

```bash
# WHOIS NetRange → ip calculation
jdan whois 8.8.8.8 --json | jdan json path "parsed.netrange" -r

# DNS A record → get IP → check if it's internal
ip=$(jdan dns lookup myserver.com -t A | tail -1)
jdan ip contains 10.0.0.0/8 "$ip" && deploy-internal
```

### `jdan cert`

Generate a self-signed TLS certificate for local development / testing. Complements `jdan ssl cert` (which inspects certificates): one makes, one inspects. Zero new dependencies (pure `crypto/x509` + `crypto/tls`).

Detailed technical docs: [docs/jdan-cert.md](docs/jdan-cert.md)

**Why a dedicated command**: local HTTPS debugging needs a self-signed cert, and nobody remembers `openssl req`'s flags, plus it's easy to forget the SAN (modern browsers won't accept a bare CN). `jdan cert localhost` does it in one line, with the correct SAN included by default.

> ⚠ For local development / testing only, do not use in production (production certs go through ACME / certbot).

```bash
$ jdan cert localhost
Generated self-signed certificate:
  Cert:        cert.pem
  Key:         cert-key.pem
  Subject:     CN=localhost
  SAN:         DNS:localhost
  Key type:    EC (P-256)
  Valid:       2026-06-16 → 2028-09-18 (825 days)
  Fingerprint: SHA256:...

# SAN auto-inferred (IP literals go into an IP SAN, otherwise DNS)
$ jdan cert myapp --ip 127.0.0.1,::1 --san "*.myapp.local"
  SAN:         DNS:myapp, DNS:*.myapp.local, IP:127.0.0.1, IP:::1

# --ca: generate a CA + leaf; trust the CA once and everything it signs is trusted
$ jdan cert localhost --ca
  CA cert:     ca.pem        ← add this to your trust store (once)
  ...

# make → inspect loop
$ jdan cert localhost && jdan ssl cert -f cert.pem
```

`--key-type ec/rsa/ed25519`, key file permissions are **0600**, with `--stdout` / `--json` output. The leaf carries a ServerAuth EKU and can be fed directly to a local HTTPS server.

### `jdan pem`

Inspect a PEM file offline: split out every PEM block, identify its type, and summarize each. **No network, never prints private key material**. Complements `jdan ssl cert` (fetches a host's TLS cert over the network) and `jdan cert` (generates a self-signed cert) — `pem` reads a local file. Zero new dependencies (reuses `internal/sslcert` for the cert description).

Detailed technical docs: [docs/jdan-pem.md](docs/jdan-pem.md)

```bash
$ jdan pem fullchain.pem
[1] CERTIFICATE  (1187 bytes)
    subject:  CN=example.com
    issuer:   CN=R3, O=Let's Encrypt
    validity: 2026-01-01 → 2026-04-01  (剩余 64d)
    SAN:      example.com, www.example.com
    key:      RSA 2048
    CA:       false
    sha256:   ab:cd:ef:...

$ jdan pem server.key
[1] PRIVATE KEY  (138 bytes)
    key:      EC P-256   （私钥内容不打印）

# when a cert + key are combined, it compares public keys to tell you if they match
$ cat cert.pem key.pem | jdan pem | tail -1
✓ 私钥与证书匹配
```

Supported block types: CERTIFICATE / CERTIFICATE REQUEST (CSR) / private keys (type + size only) / PUBLIC KEY / encrypted private keys (flagged, not decrypted) / others (type + size). Multiple blocks (fullchain / cert+key) are all listed; a block that fails to parse is noted inline and the walk continues. `--json` for structured output. No PEM blocks / unreadable file → error.

### `jdan ssh-key`

An SSH public/private key parsing tool. Three subcommands cover **combined info / fingerprint / public-key extraction**. Sits alongside the `jdan ssl` suite as a "key/certificate inspection" tool.

Detailed technical docs: [docs/jdan-ssh-key.md](docs/jdan-ssh-key.md)

**Why a dedicated command**: `ssh-keygen`'s syntax is fragmented (`-lf` for the fingerprint, `-lf -E md5` for MD5, `-y` to extract the public key), and there's no single command to see "type + bit size + fingerprint + comment" at once. `jdan ssh-key` provides a unified interface, auto-detects public vs private key, and its fingerprint is **byte-equal** to ssh-keygen's so you can cross-verify. Zero new dependencies (`golang.org/x/crypto/ssh` is already a direct dep).

```bash
# info: takes both public and private keys, all fields in one shot
$ jdan ssh-key info ~/.ssh/id_ed25519.pub
Type:         ssh-ed25519
Algorithm:    Ed25519
Bits:         256
Comment:      quincy@macbook
Fingerprint:  SHA256:Hk8x...
MD5:          MD5:43:51:43:a1:...

$ jdan ssh-key info ~/.ssh/id_rsa.pub     # RSA shows the real bit size (computed from the modulus)
Type:         ssh-rsa
Algorithm:    RSA
Bits:         4096
...

# encrypted private key: identifies without decrypting, doesn't leak key material
$ jdan ssh-key info ~/.ssh/id_ed25519     # passphrase-protected
Type:         OpenSSH private key
Encrypted:    yes (passphrase-protected; cannot derive public key without it)

# fingerprint: byte-equal to ssh-keygen -lf
$ jdan ssh-key fingerprint ~/.ssh/id_ed25519.pub
256 SHA256:Hk8x... quincy@macbook (ED25519)
$ jdan ssh-key fingerprint ~/.ssh/id_rsa.pub --md5
4096 MD5:43:51:... quincy@macbook (RSA)

# pubkey: reconstruct the public key from the private key (= ssh-keygen -y), useful when you've lost the .pub file
$ jdan ssh-key pubkey ~/.ssh/id_ed25519
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... quincy@macbook
```

**Supports** Ed25519 / RSA / ECDSA (p256/384/521) + FIDO/U2F hardware keys (`sk-*`). Input takes a file path / `-` for stdin / a pasted public-key string directly.

**Typical use**: verify a local key is the same one registered on GitHub / GitLab / a server:

```bash
jdan ssh-key fingerprint ~/.ssh/id_ed25519.pub
# → 256 SHA256:Hk8x...  ← compare against what GitHub Settings → SSH keys shows
```

### `jdan version`

Show the current binary's version, build commit, build time, and platform. Release binaries get these injected at build time by GoReleaser via `-ldflags`; binaries compiled with `go install` or a local `go build` show `dev / none / unknown`, which is an intentional fallback.

```bash
$ jdan version
jdan v0.1.0 (commit abc1234, built 2026-06-12T10:00:00Z, darwin/arm64)

$ jdan version --short
v0.1.0
```

`--short` is handy for capturing the version in a script:

```bash
JDAN_VER=$(jdan version --short)
echo "running jdan $JDAN_VER"
```

### `jdan file bak`

Copies an **ordinary file** to a backup file in the **same directory**, with this naming rule:

- Without `--desc` (or empty after trimming): `{full original filename}.bak.{YYYYMMDD-HHMMSS}`
- With `--desc`: `{full original filename}.bak.{YYYYMMDD-HHMMSS}-{description}`  
  Description: only **English letters, ASCII digits, Chinese characters, and ASCII spaces** are allowed; spaces become `_`. Any other character (punctuation, tabs, etc.) **aborts execution** and logs the reason.
- If the target backup path already exists (same timestamp): it **does not copy**, and reports "a backup with the same timestamp already exists".

```bash
jdan file bak ./report.pdf
jdan file bak ./report.pdf --desc "before edit"
```

### `jdan zip`

Pack a given **file** or **directory** into a zip archive. The output file is named `{source name}.zip` and written to the **current working directory** (not the source's directory).

```bash
jdan zip ./report.pdf      # produces report.pdf.zip in the CWD
jdan zip ./my-project      # recursively compresses the directory, produces my-project.zip
jdan zip /tmp/data         # absolute paths work too, output still goes to the CWD
```

| Parameter | Description |
|------|------|
| `path` | file or directory path (required) |

Implementation details:

- Uses the Go standard library `archive/zip` with the `Deflate` compression method
- For directories it recurses, using the source directory's basename as the root inside the zip
- No passwords, no exclusion rules, no custom output name — keeps a single responsibility
- Doesn't depend on the system `zip` binary, consistent across platforms

### `jdan http timing`

Measure the time spent in each phase of an HTTP request: DNS lookup, TCP connect, TLS handshake, server processing, content transfer, total time, plus the HTTP status code.

```bash
jdan http timing https://github.com
jdan http timing https://github.com -n 3        # request 3 times, print each result plus the average
jdan http timing https://github.com --json       # JSON output
jdan http timing https://github.com -n 3 --json  # 3 times + JSON
jdan http timing https://example.com -k          # skip TLS certificate verification
```

| Parameter | Description |
|------|------|
| `-n` | number of requests (default 1; appends an average when greater than 1) |
| `--json` | output as JSON (Duration is a float in milliseconds) |
| `-k` / `--insecure` | skip TLS certificate verification |

### `jdan http headers`

Fetch a URL and print the **status line + response headers + the full redirect chain** (hop by hop). More readable than `curl -I`. Zero new dependencies (pure stdlib). Complements `http timing` (which times the phases).

Detailed technical docs: [docs/jdan-http-headers.md](docs/jdan-http-headers.md)

```bash
$ jdan http headers http://github.com
301 Moved Permanently
  Location: https://github.com/
→ 200 OK
  Content-Type: text/html; charset=utf-8
  Server: github.com
  Strict-Transport-Security: max-age=31536000; includeSubdomains; preload
  ...

$ jdan http headers github.com               # no scheme → https:// added
$ jdan http headers <url> --max-redirects 0  # don't follow redirects
$ jdan http headers <url> -H "Authorization: Bearer x"
$ jdan http headers <url> --json
```

It **follows redirects manually** (not via the client's auto-follow), showing each hop's status/Location/headers — something auto-follow can't do. Defaults to GET but only reads the response headers, never downloading the body (sidestepping HEAD's quirks). Relative `Location` is resolved correctly; redirect loops are capped by `--max-redirects`; on a connection failure the hops that did succeed are still listed. Redirect hops show only `Location` by default; `-a` shows all headers on every hop.

### `jdan http serve`

A temporary static file server. **Core actions**: find a free port (falling back from 8080) → detect the LAN IP (RFC1918 private range) → print a QR code of the LAN URL in the terminal (reusing `jdan qr`'s renderer) → watch the access log → Ctrl+C for a graceful shutdown with a summary. **Use cases**: mac → phone file transfer, sharing a build artifact with a colleague, temporarily handing out an installer.

```bash
$ jdan http serve ~/Downloads

⚠  serving on all interfaces (0.0.0.0:8080) — anyone on your LAN can read these files
   to limit to localhost: --bind 127.0.0.1

serving /Users/quincy/Downloads on:
  http://localhost:8080
  http://192.168.10.16:8080

  █▀▀▀▀▀█ ▄ ▄ ▀▄█ █▀▀▀▀▀█
  █ ███ █  ▄▄ ▀  █ ███ █     ← QR code for 192.168.10.16:8080
  █ ▀▀▀ █ ▀▄█▄▀▀▄ █ ▀▀▀ █
  ▀▀▀▀▀▀▀ ▀▄█▄▀▄█ ▀▀▀▀▀▀▀
  ...

press Ctrl+C to stop

[GET] 200 /             127.0.0.1     12ms  (3.2KB)
[GET] 200 /report.pdf   192.168.10.42 78ms  (124.3KB)  ← downloaded after scanning on a phone
^C

served 2 request(s) to 2 client(s), 127.5KB total
```

**Key design**:

- **Defaults to `--bind 0.0.0.0`** (LAN-reachable), and prints a prominent ⚠ warning at startup to flag the risk. `--bind 127.0.0.1` opts out. This is the convention of `python -m http.server` / `npx serve` and friends
- **Auto-finds a free port**: tries 8080 up to 8129 by default, falling back to a kernel-assigned random port on failure
- **LAN IP detection is purely local**: iterates `net.Interfaces()` filtering out loopback/down/IPv6 link-local, picking an RFC1918 private address. **No network** (unlike `jdan pubip4`, which queries the public internet)
- **The QR code uses the first LAN IP** (home WiFi is usually `192.168.1.x`, which ranks above `10.x` and `172.16-31.x`)
- **Single-file serve**: `jdan http serve report.pdf` automatically serves the parent directory, with the root path `/` redirecting to `/report.pdf`
- **Directory traversal protection**: `http.FileServer` has built-in `..` path cleaning + a check for symlinks escaping the root (it normalizes via `filepath.EvalSymlinks` and compares the prefix, with special handling for the macOS `/var` → `/private/var` symlink)
- **Graceful shutdown**: SIGINT/SIGTERM triggers `http.Server.Shutdown(5s)`, so in-flight downloads aren't cut off
- **`--upload` bidirectional mode**: once enabled, `POST /upload` receives a multipart form and writes to `<root>/uploads/`, adding a timestamp suffix to same-named files to prevent overwrites; `GET /upload` returns a mobile-friendly HTML form so a phone browser can pick files

flags:

| flag | default | effect |
|------|------|------|
| `--port` | 0 (auto) | force a port, otherwise 8080 → +1 → random |
| `--bind` | `0.0.0.0` | bind address |
| `--no-qr` | false | don't print the terminal QR code |
| `--upload` | false | enable `POST /upload` + the upload form |
| `--upload-dir` | `<root>/uploads` | where uploaded files land |
| `--auth` | none | Basic Auth `user:pass` |
| `--quiet` | false | don't print the access log |
| `--json` | false | output the access log as ndjson (one event per line) |

**Deliberately out of scope**:

- TLS / HTTPS — the self-signed cert UX keeps getting worse (modern browser warnings scare people off), so leave HTTPS to a reverse proxy. A 5-minute share-and-download isn't worth the complexity
- Auto-opening a browser — the server scenario is often over ssh with no browser; copying the URL manually isn't a hassle

#### macOS firewall: LAN connections refused

**Symptom**: after `jdan http serve` starts, **`http://localhost:8080` works locally**, but **accessing via the LAN IP (like `http://192.168.1.42:8080`) gives "Connection Refused"**.

**Cause**: macOS's built-in Application Firewall blocks inbound connections to **any binary not signed by an Apple Developer** by default. jdan isn't Apple-signed even when downloaded from GitHub Releases (the Apple Developer Program is $99/year, which open-source tools generally don't pay for), so it's denied by default. `localhost` goes over lo0 and bypasses the firewall, which is why it works locally.

The startup banner detects this automatically and prints a hint:

```
⚠  serving on all interfaces (0.0.0.0:8080) — anyone on your LAN can read these files
   to limit to localhost: --bind 127.0.0.1
ℹ  macOS firewall is on; unsigned binaries may be blocked from LAN access.
   if LAN clients get "connection refused", see README §macOS firewall.
```

**Two fixes**:

**Option 1: temporarily turn off the firewall (fastest for testing)**

```bash
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --setglobalstate off
# be sure to restore it after testing:
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --setglobalstate on
```

**Option 2: allowlist jdan (sustainable, recommended)**

```bash
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add $(which jdan)
sudo /usr/libexec/ApplicationFirewall/socketfilterfw --unblockapp $(which jdan)
```

If the binary path changes (e.g. a fresh `go install`, switching to the brew version), you have to `--add` it again.

You can also use the GUI: **System Settings → Network → Firewall → Options →** click `+` to add the jdan binary → set it to **"Allow incoming connections"**.

**A real fix** requires Apple Developer signing + notarization, which isn't something jdan should do at this point. The same problem affects `python3 -m http.server`, `npx serve`, and self-built Rust binaries too.

### `jdan net probe`

Probe a target host/port/URL phase by phase from the client's perspective, with real-time output for five phases: **DNS → TCP → tcp_health → TLS → HTTP**. **Every failure carries a prominent `ErrorClass` label** so you can identify "what kind of problem" in 0.5 seconds, paired with a medium-length **"what it means"** explanation and a targeted hint. **Use case**: when you hit "can't connect / connection refused / cert error / getting kicked", pinpoint which layer is failing within 30 seconds.

```
$ jdan net probe https://github.com

✓ resolve      github.com → 1 record(s) via system resolver
  A     140.82.113.4
  duration: 8ms

✓ tcp          connected to 140.82.113.4:443 in 38ms
  ✓ 140.82.113.4 from 192.168.10.16:54321 (38ms)

✓ tcp_health   held 1s without remote close (healthy)

✓ tls          TLS 1.3, cert: github.com (issued by Sectigo, exp 2026-08-02)
  ALPN=h2, SNI=github.com, duration=142ms

✓ http         HEAD HTTP/2.0, 200 OK
  server: github.com
  content-length: 56012
  duration: 312ms

✓ all green · total 1521ms
```

On failure it shows the `ErrorClass` label + three layers of info (label / what it means / what to check):

```
$ jdan net probe 127.0.0.1:1

✓ resolve      127.0.0.1 (literal IP)
✗ tcp          CONNECTION_REFUSED
  ✗ 127.0.0.1: dial tcp 127.0.0.1:1: connect: connection refused

  what it means:
    target host received our SYN but responded with RST.
    either no process is listening on this port, or a host-level
    firewall is actively rejecting connections.
  raw error: dial tcp 127.0.0.1:1: connect: connection refused

  what to check:
    • target host not listening (check: lsof -i :PORT on target)
    • OS firewall blocking (macOS App Firewall, ufw, Windows Defender)
    ↳ run `jdan net selfcheck :PORT` on the target host to investigate

✗ failed at tcp · total 287µs
```

#### ErrorClass classification list

probe classifies failures by **protocol layer + user-facing semantics**, sparing you from guessing the cause from Go's internal error strings. The full class table:

| Phase | Class | Meaning |
|------|-------|------|
| **resolve** | `DNS_NO_SUCH_HOST` | domain doesn't exist |
| | `DNS_RESOLVER_UNREACHABLE` | can't reach the DNS server |
| | `DNS_TIMEOUT` | DNS query timed out |
| **tcp** (failed to establish a connection) | `CONNECTION_REFUSED` | got an RST: nothing listening on the port / firewall reject |
| | `CONNECTION_TIMEOUT` | no response to the SYN: firewall silently dropping |
| | `NO_ROUTE_TO_HOST` | what you'd call "no link": the router returns unreachable |
| | `NETWORK_UNREACHABLE` | local network down / no default route |
| **tcp_health** (closed by the remote) | `REMOTE_RESET_AFTER_CONNECT` | RST immediately after the TCP connection is established: **stateful firewall / IPS / anti-scraping** |
| | `REMOTE_CLOSED_AFTER_CONNECT` | got a FIN: service draining / protocol mismatch |
| **tls** | `TLS_CERT_INVALID` | self-signed / expired / SAN mismatch |
| | `TLS_HANDSHAKE_FAIL` | protocol mismatch / man-in-the-middle cutting it off |
| | `TLS_PLAIN_HTTP_ON_TLS_PORT` | using https:// to reach a plain HTTP service |
| **http** | `HTTP_4XX` / `HTTP_5XX` | application-layer error (the connection itself is healthy) |
| | `HTTP_PROTOCOL_ERROR` | protocol-level failure |

**Classification algorithm** (class.go): it first compares errnos with `errors.Is(err, syscall.ECONNREFUSED)` and friends (the most stable across Go versions), then the `net.Error.Timeout()` interface, and finally falls back to string-keyword matching.

#### The tcp_health phase: detecting "closed by the remote immediately"

After the TCP three-way handshake succeeds, it **holds for 1s by default without sending data, watching whether the remote will actively RST/FIN**. This is a semantic plain curl can't reveal — curl just shows "connection reset" and can't tell whether you got kicked right after the TCP connection or after sending the HTTP request. tcp_health classifies the first case separately as `REMOTE_RESET_AFTER_CONNECT`, commonly seen with:

- **Anti-scraping / security appliances** (CDN WAF, IPS) making a policy decision on the source IP after the SYN-ACK and then RST-ing
- **Cloud LB health-check failures** causing traffic to be dropped
- **Reverse-proxy IP allowlists** rejecting your source IP

tcp_health also recognizes a **server banner** (SSH/SMTP/POP3 send a welcome line right after accept):

```
✓ tcp_health   server pushed banner (12 bytes): SSH-2.0-OpenSSH_8.0
```

A banner isn't an error — the target you're probing simply isn't an HTTP service.

#### flags

| flag | default | effect |
|------|------|------|
| `--timeout` | 10s | per-phase timeout |
| `--resolver` | system | specify a DNS server (`host[:port]`) |
| `--method` | HEAD | HTTP method; on a 405 it auto-falls-back to GET once |
| `-k` / `--insecure` | false | skip TLS certificate verification |
| `-v` / `--verbose` | false | show the cert chain + all response headers |
| `--json` | false | structured output (includes Class / Explanation / Hint fields) |
| `--no-health` | false | skip the tcp_health phase (saves 1s, for scripts) |
| `--health-duration` | 1s | how long the tcp_health phase holds |

**Supported target forms**:

| Form | Inferred as |
|------|------|
| `https://github.com` | https + 443 |
| `example.com` | https + 443 (no scheme defaults to https) |
| `example.com:80` | http + 80 (port infers the scheme) |
| `192.168.1.42:8080` | http + 8080 |
| `[::1]:8080` | IPv6 literal |

**Design notes**:

- **TCP-connects each IP serially**, not using Go's default Happy Eyeballs. The whole value of a probe tool is showing the specific result for each IP (a "IPv4 works but IPv6 doesn't" problem gets hidden by Happy Eyeballs)
- **HEAD by default**, auto-falling-back to GET on a 405 (many servers don't support HEAD)
- **errno-based error classification**: `errors.Is(err, syscall.ECONNREFUSED)` and the like are more stable across Go versions than string-keyword matching, with string matching as the fallback
- **cross-references `jdan net selfcheck`**: when you can't connect, it guides you to run the self-check on the server
- The exit code is always 0 (the probe command itself ran fine); to detect "did the probe pass" use `--json` and check the `.ok` field

### `jdan net selfcheck`

Diagnostics from the server's perspective: "should I, as a server, be reachable from outside?" Paired with `jdan net probe`: when probe finds it can't connect on the client side, the hint tells the user to run `jdan net selfcheck :PORT` on the server.

```
$ jdan net selfcheck :8080

◇ os & firewall
  • darwin/arm64
  ⚠ Application Firewall: ON
    macOS App Firewall is ON. unsigned binaries (like jdan) may be blocked.
      fix:
        sudo /usr/libexec/ApplicationFirewall/socketfilterfw --add $(which jdan)
        sudo /usr/libexec/ApplicationFirewall/socketfilterfw --unblockapp $(which jdan)

◇ network interfaces
    lo0 (loopback)
      127.0.0.1/8
    en0 (LAN)
      192.168.10.16/24
  ★ utun1024 (primary)
      198.18.0.1/30

◇ listening on :8080
  ✓ jdan (pid 29377, user quincy) bind=127.0.0.1:8080 (localhost-only)

◇ self-loop test
  ✓ http://127.0.0.1:8080 in 1ms

◇ prediction
  port :8080 is bound to loopback only (127.0.0.1 or ::1).
    external clients CANNOT reach this. server must bind 0.0.0.0 or specific LAN IP.
```

**What it checks**:

| Check | How |
|------|------|
| OS / architecture | `runtime.GOOS` / `runtime.GOARCH` |
| macOS firewall status | exec `socketfilterfw --getglobalstate` (reuses `internal/sysprobe`) |
| network interface list | `net.Interfaces()` + tags LAN / loopback / **★ primary** (the default-route egress) |
| port listening status (when a port is given) | exec `lsof -iTCP:PORT -sTCP:LISTEN` to see the process, PID, user, bind address |
| whether the `bind` is LAN-reachable or localhost-only | distinguishes `0.0.0.0` / `*` / a specific LAN IP (reachable) vs `127.0.0.1` / `::1` (local only) |
| self-loop test | HTTP GET `http://localhost:PORT` and `http://<primary LAN IP>:PORT` |
| prediction | combines all of the above into a one-line "external clients can/can't reach this" verdict + a fix path |

**CLI**:

```bash
jdan net selfcheck                 # general diagnostics (doesn't check a specific port)
jdan net selfcheck 8080            # explicit port
jdan net selfcheck :8080           # same (the colon is optional)
jdan net selfcheck 8080 --json     # structured output
```

**Typical prediction scenarios**:

| Situation | What prediction says |
|------|------|
| firewall off + bind 0.0.0.0 + self-loop works | "LAN-reachable from self. external clients should reach ..." |
| firewall ON + bind 0.0.0.0 | "LAN-reachable, BUT firewall is on; clients may see 'connection refused', apply fix above" |
| bind 127.0.0.1 | "bound to loopback only ... external clients CANNOT reach this. server must bind 0.0.0.0" |
| nothing listening on the port | "nothing is listening on :PORT. start your server first." |
| lsof not present | "can't determine if anyone is listening on :PORT (install lsof to enable)." |

**Dependencies**:

- macOS / mainstream Linux ship with `lsof` by default. Minimal environments like Alpine may not have it, and selfcheck degrades gracefully with an `install lsof` hint
- Only macOS has real application-layer firewall detection; Linux/Windows aren't implemented yet (iptables/ufw/Defender semantics differ too much)

### `jdan ssl cert`

Inspect an HTTPS host's certificate details: the full chain + three verification checks (trust/hostname/expiry) + an OCSP revocation-status lookup. **Use cases**:

- See how long until the cert expires (with a progress bar)
- See which domains the cert covers (SAN)
- See the full chain to troubleshoot a missing intermediate
- See the fingerprint for cert pinning
- Inspect a local PEM file (no network)
- Monitoring scripts: `--expires-in 30d` triggers exit 1

```
$ jdan ssl cert github.com

╭─ leaf ──────────────────────────────────────────────────────────────╮
│ Subject:    CN=github.com                                          │
│ Issuer:     CN=Sectigo Public Server Authentication CA DV E36,...  │
│ Valid:      2026-05-05 → 2026-08-02  (89d total)                   │
│ Days left:  █████░░░░░  50 days                                    │
│ SAN:        github.com, www.github.com                             │
│ Key:        EC P-256                                               │
│ Signed:     ECDSA-SHA256                                           │
│ Serial:     e7:ce:cc:3b:13:fb:3b:7b:8a:46:ea:8c:d0:ae:b7:1c        │
│ SHA256:     a7:b8:10:34:cd:43:95:51:c...9e:12:85:6c:85:5b:64:b6:5f │
╰────────────────────────────────────────────────────────────────────╯

Chain:
  ▸ leaf:        CN=github.com  (exp in 50d)
  ▸ intermediate: CN=Sectigo Public Server Authentication CA DV E36  (exp in 3569d)
  ▸ root:        CN=Sectigo Public Server Authentication Root E46  (exp in 7221d, self-signed)

Verification:
  ✓ chain trusted (system trust store)
  ✓ hostname matches SAN
  ✓ not expired

OCSP:
  ✓ CN=github.com  OCSP good
  ✓ CN=Sectigo Public Server Authentication CA DV E36  OCSP good
```

**flags**:

| flag | default | effect |
|------|------|------|
| `-f` / `--file` | none | read from a local PEM file, no network |
| `--sni` | host | the SNI sent in the TLS handshake (virtual-host scenarios) |
| `--full` | false | expand extensions / KeyUsage / OCSP URL etc. |
| `--json` | false | structured output (includes Verification + OCSP fields) |
| `--pem` | false | output standard PEM for piping |
| `--no-ocsp` | false | skip OCSP (saves ~300-500ms) |
| `--timeout` | 5s | overall timeout |
| `--expires-in` | none | e.g. `30d` / `720h`; exit 1 if the leaf expires within this window |

**Key design**:

- **`InsecureSkipVerify` to fetch the cert, but verify separately**: if you want to "inspect a certificate" you can't reject it just because it's untrusted. The fetch phase ignores trust to get the full chain, while the verify phase separately runs the system trust store + hostname + expiry checks, and shows the result to the user as a report
- **errno-based OCSP**: uses `golang.org/x/crypto/ocsp` (quasi-stdlib); silently skips when a cert has no OCSP responder URL (common for root certs); on network failure it adds a `⚠` warning but doesn't reject the command
- **Expiry countdown progress bar**: `█████░░░░░  50 days` — see at a glance "how long this cert has left to live", friendlier than the wall of ASCII from `openssl x509 -text -noout`
- **Expiry-detection for scripts**: `--expires-in 30d` lets a monitoring script do `if ! jdan ssl cert host --expires-in 30d; then alert; fi`
- **Reuses the internal/sslcert/ package**: `internal/netprobe/tls.go` can later be upgraded to use the same Describe for SAN, at zero extra cost
- **No OCSP stapling** (grabbing the stapled response from the TLS handshake): complex, low coverage; querying the OCSP responder directly is more reliable. **No CRL either**: large files, narrow use case
- **DSA algorithm recognition**: modern certs barely use DSA, so falling through to the `PublicKeyAlgorithm.String()` fallback is fine

**Deliberately out of scope**:

- `jdan ssl diff a b` to compare two hosts' certs
- `jdan ssl watch` for continuous monitoring
- `jdan ssl ct` to query Certificate Transparency logs
- CRL revocation checks (OCSP is enough)
- OCSP stapling parsing

### `jdan ssl scan`

A full TLS configuration audit: it runs five blocks of checks on an HTTPS host (version / cipher / ALPN / HSTS / session reuse / cert) and gives an A+/A/B/C/D/F grade weighted across 5 ssllabs-style dimensions. **Use cases**:

- A local replacement for ssllabs.com on an **internal / private host**
- A CI/CD security gate: `--grade-only` outputs the grade letter, exit 1 below C
- Ops quickly answering "is my server's config secure?"
- Before/after comparison after upgrading a TLS configuration

```
$ jdan ssl scan github.com

╭─ TLS Versions ─────────────────────────────────────────────╮
│ ✗ TLS 1.0   refused    (recommended off)                 │
│ ✗ TLS 1.1   refused    (recommended off)                 │
│ ✓ TLS 1.2   supported                                    │
│ ✓ TLS 1.3   supported (preferred)                        │
╰──────────────────────────────────────────────────────────╯

╭─ Cipher Suites (TLS 1.2) ──────────────────────────────────╮
│ ✓ TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384    (strong)      │
│ ✓ TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305   (strong)      │
│ ✓ TLS_RSA_WITH_AES_256_GCM_SHA384  (acceptable; no forward sec) │
│                                                          │
│ Weak ciphers correctly refused:                          │
│   ✓ TLS_RSA_WITH_3DES_EDE_CBC_SHA refused                │
│                                                          │
│ TLS 1.3 ciphers are mandatory (5 fixed suites); not enumerated │
╰──────────────────────────────────────────────────────────╯

╭─ HTTP Stack ───────────────────────────────────────────────╮
│ ALPN:    h2, http/1.1                                    │
│ HSTS:    max-age=31536000; includeSubdomains; preload    │
│          strength=preload, max-age=31536000              │
╰──────────────────────────────────────────────────────────╯

╭─ Cert ─────────────────────────────────────────────────────╮
│ Subject:    CN=github.com                                │
│ Key:        EC P-256                                     │
│ Days left:  49                                           │
│ Chain:      trusted ✓                                    │
│ Hostname:   matches SAN ✓                                │
╰──────────────────────────────────────────────────────────╯

Overall: A+  (100/100)

Strong points:
  ✓ certificate trusted and valid
  ✓ TLS 1.3 supported
  ✓ TLS 1.3 enforces forward secrecy
  ✓ 6 modern cipher(s) supported (AES-GCM/ChaCha20)
  ✓ HSTS with preload (1y + subdomains + preload list)
  ✓ HTTP/2 supported via ALPN
```

**Grading logic** (inspired by the ssllabs SSL Server Test):

| Dimension | Weight | Judged on |
|------|------|------|
| Cert | 25 pts | trusted + valid + key ≥ 2048 + sig ≠ SHA1 |
| Protocol | 30 pts | TLS 1.3 +30 / 1.2 +20 / 1.1 -15 / 1.0 -20 |
| Key Exchange | 25 pts | Forward Secrecy (ECDHE / DHE) |
| Cipher Strength | 20 pts | RC4/DES/3DES deduct; AES-GCM/ChaCha20 add |
| Modifiers | bonus | HSTS preload +5 / HSTS good +3 / H2 +2 / resume +1 |

Mapping: 90+ A+ / 80+ A / 65+ B / 50+ C / 35+ D / < 35 F

**flags**:

| flag | default | effect |
|------|------|------|
| `--sni` | host | the server_name sent in the TLS handshake |
| `--full-cipher` | false | try 40 ciphers instead of the 16 common ones (slower) |
| `--no-cipher` | false | skip cipher enumeration (fastest) |
| `--no-hsts` | false | skip the HSTS HTTP GET |
| `--no-resume` | false | skip the session-resumption test |
| `--json` | false | structured output |
| `--grade-only` | false | output only the grade letter; exit 1 below C (for CI/CD) |
| `--timeout` | 15s | overall timeout |

**Design notes**:

- **Per-version independent handshake**: uses `MinVersion=MaxVersion` to force a single version; a server failure = unsupported. More reliable than "asking the server for its supported list"
- **TLS 1.3 ciphers not enumerated**: the protocol mandates 5 fixed suites, so there's no point
- **No SSL 3.0**: Go stdlib already removed it, and it's extinct in production
- **No cryptographic assessment**: uses a static classification table (RC4/DES = weak, AES-GCM = strong). jdan isn't a cryptographic audit tool, it's a configuration audit tool
- **HSTS via an HTTPS GET to grab the header**: failure doesn't affect the grade (marked "not configured")
- **CI/CD gate**: `--grade-only` lets `if ! jdan ssl scan host --grade-only; then alert; fi` hook into monitoring in one line
- **Reuses internal/sslcert/**: the cert block uses the same fetch + Describe, at zero extra code

**Deliberately out of scope**:

- The kind of public testing + shared caching SSL Labs does
- Real cryptographic algorithm strength assessment
- Certificate Transparency log queries
- Client cert / mTLS testing
- HTTP/3 (QUIC) support (QUIC runs over UDP, outside the TCP+TLS scope)

### `jdan ssl pin`

Generate the SPKI hash for cert pinning, matching the mainstream cert-pinning formats: **OkHttp (Android)** / **iOS NSAppTransportSecurity** / **HPKP HTTP header** / **Mozilla NSS** / **curl `--pinnedpubkey`** / raw base64.

#### ⚠ Important: SPKI hash ≠ cert fingerprint

cert pinning **must not use the cert fingerprint** (i.e. the SHA256 shown by `jdan ssl cert`); it must use the **SPKI hash**:

| Concept | Formula | Use |
|------|------|------|
| Certificate fingerprint | `SHA256(cert.Raw)` | hash of the entire cert content |
| **SPKI hash** | `SHA256(cert.RawSubjectPublicKeyInfo)` | **use this for cert pinning** |

certs are renewed often (same key), and after renewal the cert fingerprint changes and **pinning breaks**; the SPKI hash is **stable** as long as the key is unchanged. HPKP RFC 7469 / Chrome static pins / iOS Apple Doc / Android Network Security Config all uniformly use the SPKI hash.

#### Pins the leaf + first intermediate by default

The best practice recommended by Apple / Android / Chromium static pins:
- the **leaf hash** makes the pin precise
- the **intermediate hash** lets a cert renewal still match (renewals usually keep the same issuer)

`--leaf-only` opts out to the leaf only; `--full` computes every cert in the chain.

#### Sample output

```
$ jdan ssl pin github.com

╭─ Leaf ─────────────────────────────────────────────────────
│ Subject:    CN=github.com
│ Issuer:     CN=Sectigo Public Server Authentication CA DV E36
│ SPKI hash:  Ry0vLQcAM0ZpwjfCIday3P4budz0fLwe34EWXN1ZWdk=
╰──────────────────────────────────────────────────────────

╭─ Intermediate ─────────────────────────────────────────────
│ Subject:    CN=Sectigo Public Server Authentication CA DV E36
│ Issuer:     CN=Sectigo Public Server Authentication Root E46
│ SPKI hash:  ZSagvDzjltLkewXEBuDxIzpW/dpVw1Juvvmd0hhkzdY=
╰──────────────────────────────────────────────────────────

─── Pin formats ─────────────────────────────────────────────

▸ okhttp:
    CertificatePinner.Builder()
      .add("github.com", "sha256/Ry0vLQcAM0ZpwjfCIday3P4budz0fLwe34EWXN1ZWdk=")
      .add("github.com", "sha256/ZSagvDzjltLkewXEBuDxIzpW/dpVw1Juvvmd0hhkzdY=")
      .build()

▸ ios:
    <key>NSAppTransportSecurity</key>
    <dict>
      <key>NSPinnedDomains</key>
      <dict>
        <key>github.com</key>
        <dict>
          <key>NSIncludesSubdomains</key>
          <true/>
          <key>NSPinnedCAIdentities</key>
          <array>
            <dict>
              <key>SPKI-SHA256-BASE64</key>
              <string>Ry0vLQcAM0ZpwjfCIday3P4budz0fLwe34EWXN1ZWdk=</string>
            </dict>
            ...

▸ hpkp:
    Public-Key-Pins: pin-sha256="Ry0vLQc..."; pin-sha256="ZSagvDz..."; max-age=5184000; includeSubDomains

▸ nss:
    pin-sha256:Ry0vLQcAM0ZpwjfCIday3P4budz0fLwe34EWXN1ZWdk=
    pin-sha256:ZSagvDzjltLkewXEBuDxIzpW/dpVw1Juvvmd0hhkzdY=

▸ curl:
    curl --pinnedpubkey 'sha256//Ry0vLQc...=;sha256//ZSagvDz...=' https://github.com/

▸ raw:
    Ry0vLQcAM0ZpwjfCIday3P4budz0fLwe34EWXN1ZWdk=
    ZSagvDzjltLkewXEBuDxIzpW/dpVw1Juvvmd0hhkzdY=
```

#### CLI usage

```bash
jdan ssl pin github.com                        # all 6 formats by default
jdan ssl pin example.com:8443 --format okhttp  # only OkHttp, for piping
jdan ssl pin example.com --leaf-only           # only the leaf SPKI
jdan ssl pin example.com --full                # every cert in the chain
jdan ssl pin -f cert.pem                       # local PEM file
jdan ssl pin example.com --json                # structured output
```

#### flags

| flag | default | effect |
|------|------|------|
| `-f` / `--file` | none | local PEM file |
| `--sni` | host | TLS SNI |
| `--format` | all 6 | a single format: `okhttp` / `ios` / `hpkp` / `nss` / `curl` / `raw` |
| `--leaf-only` | false | compute only the leaf SPKI |
| `--full` | false | every cert in the chain |
| `--json` | false | structured output with both `entries` + `formats` sections |
| `--timeout` | 5s | TLS handshake timeout |

`--leaf-only` and `--full` are mutually exclusive; the other flags can be combined.

#### Verifying the algorithm is correct

Our SPKI hash is equivalent to OpenSSL's:

```bash
# the standard OpenSSL pipeline for computing the SPKI hash
openssl x509 -in cert.pem -pubkey -noout |
  openssl pkey -pubin -outform DER |
  openssl dgst -sha256 -binary | base64

# jdan's output should be byte-equal
jdan ssl pin -f cert.pem --format raw --leaf-only
```

The tests independently compute the equivalent SPKI hash with `crypto/x509.MarshalPKIXPublicKey` to ensure the two are byte-identical (covering all three key types: RSA / EC / Ed25519).

### `jdan dns lookup`

Query a domain's multiple DNS record types concurrently, getting full diagnostic info for A / AAAA / MX / TXT / CNAME / NS in one command. Compared to `dig`, which queries only the A record by default, `jdan dns lookup` queries the 6 most common types at once by default and sends them concurrently, so the total time ≈ the slowest single type.

```bash
jdan dns lookup example.com                       # query 6 types by default
jdan dns lookup example.com -t A                  # only the A record
jdan dns lookup example.com -t A,MX,TXT           # specify multiple types, comma-separated
jdan dns lookup example.com -t all                # query 9 types (incl. SOA / CAA / SRV)
jdan dns lookup example.com -s 8.8.8.8            # specify a DNS server (bypassing the local resolver)
jdan dns lookup example.com --json                # JSON output, for script consumption
jdan dns lookup example.com --short -t A          # values only, dig +short style
jdan dns lookup example.com --verbose             # adds query time at the top, plus an rcode column
jdan dns lookup example.com --strict              # exit 1 if any type fails
jdan dns lookup example.com --timeout 2s          # adjust the overall query timeout (default 5s)
```

| Parameter | Description |
|------|------|
| `-t` / `--type` | the record types to query, comma-separated; `all` means 9; empty means the default 6 (A / AAAA / MX / TXT / CNAME / NS) |
| `-s` / `--server` | DNS server (e.g. `8.8.8.8` or `8.8.8.8:53`); empty reads the system config from `/etc/resolv.conf` |
| `-j` / `--json` | output as JSON (includes full metadata: TTL / rcode / query_time_ms etc.) |
| `--short` | values only, one per line (good for scripts: `IP=$(jdan dns lookup example.com --short -t A)`) |
| `-v` / `--verbose` | adds query time at the top, with rcode as its own column |
| `--strict` | `exit 1` if any type fails (NXDOMAIN / SERVFAIL / TIMEOUT); default is lenient (any success → `exit 0`) |
| `--timeout` | overall query timeout (default 5s) |

Exit codes: in the default lenient mode, as long as any type returns NOERROR (including an empty record set) it's `exit 0`; only when all types fail is it `exit 1`. `--strict` switches to strict mode, where any single type's failure immediately yields `exit 1`.

By default it reads the system DNS server from `/etc/resolv.conf`, falling back to `8.8.8.8:53` if it can't. A top line prints `domain — via X.X.X.X:53` to show the actual query source, handy for confirming the query path under VPN / corporate intranet / DNS-hijacking conditions.

**Bypass local DNS hijacking via DoH (DNS-over-HTTPS, RFC 8484):**

```bash
jdan dns lookup example.com --doh google         # use Google DoH (8.8.8.8)
jdan dns lookup example.com --doh cloudflare     # use Cloudflare DoH (1.1.1.1)
jdan dns lookup example.com --doh quad9          # use Quad9 DoH (9.9.9.9)
jdan dns lookup example.com --doh dns.google     # hostname form (auto-appends /dns-query)
jdan dns lookup example.com --doh https://dns.alidns.com/dns-query  # custom full URL
```

Built-in aliases supported (6 total):

| Alias | DoH endpoint | Bootstrap IPs |
|------|--------------|----------------|
| `google` | `https://dns.google/dns-query` | `8.8.8.8` / `8.8.4.4` |
| `cloudflare` | `https://cloudflare-dns.com/dns-query` | `1.1.1.1` / `1.0.0.1` |
| `quad9` | `https://dns.quad9.net/dns-query` | `9.9.9.9` / `149.112.112.112` |
| `opendns` | `https://doh.opendns.com/dns-query` | `208.67.222.222` / `208.67.220.220` |
| `ali` | `https://dns.alidns.com/dns-query` | `223.5.5.5` / `223.6.6.6` |
| `360` | `https://doh.360.cn/dns-query` | `101.226.4.6` / `218.30.118.6` |

**The alias form** connects directly to the corresponding DoH server using the built-in Bootstrap IPs, **completely bypassing the local resolver** — this is jdan dns lookup's "see the truth" mode in a DNS-hijacked environment. The TLS SNI is still the endpoint's host name (`dns.google` etc.), and certificate verification is unchanged. The mechanism is the same as `curl --resolve`.

**The hostname / full-URL form** resolves the DoH host via the OS resolver, suitable for non-hijacked environments or custom DoH servers (including private endpoints with a UUID path like NextDNS).

`--doh` and `--server` are mutually exclusive; TLS certificates are verified by default (DoH has no `--insecure-tls`).

> Only macOS and Linux are supported; Windows is out of scope for the first release (the Windows path for resolver auto-detection needs separate implementation).

### `jdan dns reverse`

Reverse-resolve an IP to a hostname (PTR lookup). The dual of `jdan dns lookup` — the former goes "domain → info", the latter "IP → hostname".

```bash
jdan dns reverse 8.8.8.8                    # uses the system resolver by default
jdan dns reverse 8.8.8.8 --doh cloudflare   # bypass local hijacking via DoH
jdan dns reverse 1.1.1.1 --doh google       # any built-in alias (same as dns lookup)
jdan dns reverse 2001:4860:4860::8888       # IPv6 automatically uses ip6.arpa
jdan dns reverse 8.8.8.8 --short            # output only the PTR value (script-friendly)
jdan dns reverse 8.8.8.8 --json             # full metadata (includes a display_name field)
```

Supports exactly the same flags as `jdan dns lookup`: `--server` / `--doh` / `--json` / `--short` / `--verbose` / `--strict` / `--timeout`. **The only difference** is there's no `--type` — reverse only queries the PTR record type. The `--doh` aliases (`google` / `cloudflare` / `quad9` / `opendns` / `ali` / `360`) still connect directly via the built-in IPs, getting the real PTR even in a hijacked environment.

**Input requirement**: it only accepts a single IP literal (IPv4 or IPv6). The following inputs are rejected with a hint on correct usage:

| Input | Error hint |
|------|----------|
| a domain (like `google.com`) | "use `jdan dns lookup` instead" |
| a CIDR (like `8.8.8.8/32`) | "pass a single IP" |
| host:port (like `8.8.8.8:53`) | "don't pass a port" |
| link-local with a zone-id (like `fe80::1%en0`) | "not a valid IP" |

`0.0.0.0` / `127.0.0.1` / private IPs etc. aren't blocked — following the "DNS truth" principle, the query is passed through (most return NXDOMAIN), consistent with the command's diagnostic purpose.

**The top of the output** shows the original IP (`8.8.8.8 — via …`), not the `8.8.8.8.in-addr.arpa.` form. The JSON output includes a `display_name` field (the original IP) + a `domain` field (the arpa domain actually queried), so scripts can consume whichever they need.

### `jdan dns trace`

**Iterative resolution** starting from the root DNS servers, showing the delegation path at each hop (jdan's take on `dig +trace`). `jdan dns lookup` is "ask a recursive resolver for the final answer", `jdan dns trace` is "walk the whole way yourself hop by hop, seeing how each NS hands you off to the next".

```bash
jdan dns trace example.com                  # trace from the 13 roots, querying A by default
jdan dns trace example.com -t NS            # --type override (dig +trace style)
jdan dns trace example.com --doh google     # glueless NS goes through DoH bootstrap (bypassing local hijacking)
jdan dns trace example.com --short          # only the final answer
jdan dns trace example.com --json | jq '.hops | length'  # script consumption
jdan dns trace example.com --verbose        # each hop includes NS referrals and glue details
jdan dns trace example.com -s 1.1.1.1       # use a recursive resolver as the starting server
jdan dns trace example.com --hop-timeout 2s --timeout 15s
```

**The core differences from `jdan dns lookup`**:

| | `dns lookup` | `dns trace` |
|---|--------------|-------------|
| Query model | a single query to a recursive resolver | multi-hop, iteratively tracing the authoritative NS from the root |
| DoH | `--doh` switches the entire query to HTTPS | `--doh` is used **only** for glueless NS bootstrap; the main hop path still queries the authoritative NS directly over UDP/TCP |
| `--server` | a DNS resolver IP | the starting NS IP (overrides the 13 roots) |
| Default type | 6 types concurrently | A only, `--type` overrides (dig style; multiple types make the chain ×6) |
| When to use | "where does this domain / IP resolve to right now" | "how does the delegation chain go, which hop is slow, is the NS delegation correct, am I being locally hijacked" |

**Hijack detection (important)**: trace has a built-in sanity check — a root server should return a REFERRAL, not an ANSWER, for a non-root domain query. On a network where the gateway intercepts UDP-53 (even traffic sent to the root server IPs gets forged responses), a first hop that returns an ANSWER directly is flagged as a "suspicious response" and marked ERROR, prompting the user to switch to `jdan dns lookup --doh google` for an HTTPS-encrypted query. This is what keeps trace **honest** on a polluted network.

**The semantics of `--strict` in trace**: by default, getting the final answer is `exit 0` (even if some root server timed out mid-way and was fallen back from). `--strict` switches to "any hop erroring → `exit 1`" — for diagnosing "which hop is unstable".

> Only macOS and Linux are supported; consistent with `dns lookup` / `reverse`.

### `jdan ping`

ping a host, but with `--dns` you can **pick which DNS server resolves the hostname**: if given, it first resolves the name to an IP via that DNS, then pings the IP; if not, it falls back to the system ping's default behavior. Zero new dependencies.

Detailed technical docs: [docs/jdan-ping.md](docs/jdan-ping.md)

The system `ping <hostname>` only uses the system resolver and has no `--dns` option. `jdan ping` folds "resolve via a chosen DNS + ping" into one step, so when chasing DNS hijacking / different DNS resolving to different IPs you can see the result directly.

```bash
$ jdan ping --dns 8.8.8.8 example.com
example.com → 93.184.216.34 (via 8.8.8.8)     # resolution header jdan adds
PING 93.184.216.34 (93.184.216.34): 56 data bytes   # system ping output, verbatim
64 bytes from 93.184.216.34: icmp_seq=0 ttl=56 time=12.1 ms

$ jdan ping example.com                       # no --dns → system ping's default behavior
$ jdan ping --doh google example.com          # DoH alias (carries bootstrap IPs, stronger against hijacking)
$ jdan ping --dns 8.8.8.8 -c 3 example.com --json
$ jdan ping --dns 8.8.8.8 example.com -- -i 0.2 -s 64   # args after -- pass through to system ping
```

**Two ways to pick the resolver**: `--dns` (`8.8.8.8` / `host:port` / full DoH URL) and `--doh` (alias `google`/`cloudflare`/… / hostname / URL), which are **mutually exclusive**. `--doh <alias>` is stronger because it carries **bootstrap IPs** — it bypasses local DNS even for resolving the DoH endpoint's hostname, making it the preferred choice against hijacking (in a fake-ip-hijacked network, `--dns 8.8.8.8` got a forged IP while `--doh google` got the real one via its bootstrap IPs).

**Design**: the actual ICMP is done by the system ping (shelling out, like `jdan git`); jdan only handles "resolve to an IP via the chosen DNS + build the argv + best-effort parse of the summary line for `--json`". Key point: when a resolver is specified it always pings the resolved IP, not the hostname, otherwise ping would re-resolve via the system resolver and bypass your chosen DNS. `-c` is built in (common to Linux/macOS); other advanced flags pass through after `--` without translation. macOS + Linux only (for IPv6, Linux uses `ping -6`, macOS uses `ping6`).

### `jdan pubip4` / `jdan pubip6`

Look up your machine's current outbound public IP address.

```bash
jdan pubip4                   # print the public IPv4 address (default uses ipify)
jdan pubip6                   # print the public IPv6 address (default uses ipify)
jdan pubip4 -p ipip           # use ipip.net to query IPv4
jdan pubip6 -p ipip           # use ipip.net to query IPv6
```

| Parameter | Description |
|------|------|
| `-p` / `--provider` | IP lookup service: `ipify` (default) or `ipip` |

It automatically retries up to 3 times internally, and after all failures prints a hint and exits with a non-zero exit code.

### `jdan ports`

Show all network ports currently in the LISTEN state on your machine. The table is grouped by protocol (TCP first / UDP after), sorted ascending by port number within each protocol.

```bash
jdan ports               # default table output, shows both TCP + UDP
jdan ports --tcp         # TCP only (-t)
jdan ports --udp         # UDP only (-u)
jdan ports --json        # JSON array output (-j), script-friendly
```

| Parameter | Description |
|------|------|
| `-j` / `--json` | output as a JSON array (`[{protocol, address, port, process}, ...]`) |
| `-t` / `--tcp` | show TCP ports only |
| `-u` / `--udp` | show UDP ports only |

Each record includes: `PROTOCOL`, `ADDRESS` (like `127.0.0.1`, `*`, `[::1]`), `PORT`, `PROCESS` (process name).

Implementation details:

- Under the hood it calls macOS's built-in `lsof -i -P -n -sTCP:LISTEN` (TCP) and `-sUDP:LISTEN` (UDP)
- It can show ports and addresses without sudo; the process name shows `-` when permissions are insufficient
- Ports Docker maps to the host via `-p` are detected (the host socket genuinely exists)
- It doesn't show connection states other than LISTEN (ESTABLISHED, etc.)

> Currently macOS only. Linux support is left for future extension (using `ss` or `/proc/net/{tcp,udp}` instead of `lsof`).

### `jdan macgpu`

Monitor an Apple Silicon Mac's GPU utilization, power, frequency, and thermal pressure level in real time.
Displayed in an htop/glances-style TUI: a colored ASCII bar chart on top + a details table at the bottom.

> **Requirements:** Apple Silicon (arm64) Macs only, needs `sudo` to run.

```bash
sudo jdan macgpu                # samples every 2 seconds by default
sudo jdan macgpu -i 1000        # sample every 1 second (minimum 500ms)
```

| Parameter | Description |
|------|------|
| `-i` / `--interval` | sampling interval (ms, default 2000, minimum 500) |

Press `q` to quit the TUI.

### `jdan tree2`

Show a two-level directory structure in multiple columns based on the current terminal width, showing directories only by default. Good for quickly scanning a project structure in a wide terminal, reducing the vertical scrolling of `tree -L 2`.

```bash
jdan tree2                         # view the current directory, two levels, auto-inferred column count
jdan tree2 ./internal --width 120   # specify the width, handy for scripts or reproducible tests
jdan tree2 --cols 1                 # force single-column output
jdan tree2 --files                  # include files
jdan tree2 --all                    # include hidden files and directories
jdan tree2 --limit 0                # don't limit the number of children shown per top-level directory
```

| Parameter | Description |
|------|------|
| `--cols` | the number of output columns (defaults to auto-inferred from terminal width) |
| `--width` | the terminal width (auto-detected by default, falls back to 80 on failure) |
| `--files` | include files (directories only by default) |
| `--all` | include hidden files and directories |
| `--limit` | the max number of children shown per top-level directory, default 50; `0` means no limit |

### `jdan disk`

Like `df`: list each mount point's size / used / available / use%, with a usage bar and high-usage coloring. Give a path to see just that path's filesystem. **Zero new dependencies** (pure `syscall`). darwin / linux only.

Detailed technical docs: [docs/jdan-disk.md](docs/jdan-disk.md)

```bash
$ jdan disk
文件系统         容量   已用   可用  使用率          挂载点
/dev/disk3s1s1  1.8Ti  1.6Ti  269Gi  86% ████████░   /
/dev/disk9s2    1.8Ti  1.7Ti   95Gi  95% █████████   /Volumes/m1max-tm

$ jdan disk /        # just the filesystem holding the root path
$ jdan disk -a       # include pseudo filesystems (devfs/tmpfs/map…)
$ jdan disk -i       # show inode usage
$ jdan disk --bytes  # raw bytes
$ jdan disk --json
```

The use% matches `df` (`used/(used+avail)`, rounded up). On a TTY, use% ≥90% is red and ≥75% is yellow; piped/redirected output is plain text with no ANSI. Pseudo filesystems **and TimeMachine local snapshots** are hidden by default; `-a` shows everything. Over-long device names / mount points are **middle-ellipsis truncated** to the terminal width (TTY only; piped / `--json` / `--no-trunc` show full text). Windows is not supported yet (clear error).

### `jdan unix-time`

Convert a Unix timestamp (seconds or milliseconds) to a readable time in the local timezone.

```bash
jdan unix-time 1711843200000
echo 1711843200 | jdan unix-time
```

| Rule | Description |
|------|------|
| input length 10 | parsed as a second-level timestamp |
| input length 13 | parsed as a millisecond-level timestamp |
| output timezone | the machine's local timezone |

### `jdan cal`

Print a Gregorian calendar and highlight today. Defaults to the current month, **Monday start (ISO)**, Chinese weekday headers. Zero new dependencies (pure `time`).

Detailed technical docs: [docs/jdan-cal.md](docs/jdan-cal.md)

```bash
$ jdan cal
    2026 年 6 月
一 二 三 四 五 六 日
 1  2  3  4  5  6  7
 8  9 10 11 12 13 14
15 16 17 18 19 20 21
22 23 24 25 26 27 28
29 30

$ jdan cal 12 2025      # specific month/year (cal 6 = June this year — avoids Unix cal's "6 = year 6 AD" footgun)
$ jdan cal -y 2026      # whole year (3×4 month blocks)
$ jdan cal -3           # previous / current / next month side by side
$ jdan cal -w           # show ISO week numbers in a left column
$ jdan cal 6 2026 -s    # Sunday start

$ jdan cal -l           # lunar overlay: the lunar day under each Gregorian day (1st shows the month name)
               2026 年 6 月
  一    二    三    四    五    六    日
  1     2     3     4     5     6     7
 十六  十七  十八  十九  二十  廿一  廿二
  ...
  15    16    17    18    19    20    21
 五月  初二  初三  初四  初五  初六  初七
```

Today is **reverse-highlighted** on a TTY; output is plain text (no ANSI, parseable) when piped/redirected. `--json` gives structured `{year, month, week_start, weeks}` data. `-l/--lunar` overlays the lunar calendar on the month grid (two rows per cell; the lunar month name shows on the 1st, e.g. a leap month renders as 闰六月; single month only; the 1900–2100 lunar table comes from `jdan lunar`). Single-day lunar lookup / conversion / festivals live in the separate `jdan lunar`.

### `jdan lunar`

Convert between the Gregorian and Chinese lunar calendars, with **sexagenary (ganzhi) year, zodiac, and lunar festivals**. Uses an embedded 1900–2100 lunar table (public algorithm, ~200 constants), **zero new dependencies**.

Detailed technical docs: [docs/jdan-lunar.md](docs/jdan-lunar.md)

```bash
$ jdan lunar
公历: 2026-06-26 (周五)
农历: 丙午年 五月十二  (生肖 马)

$ jdan lunar 2024-02-10              # a Gregorian date → lunar
公历: 2024-02-10 (周六)
农历: 甲辰年 正月初一  (生肖 龙)

$ jdan lunar --to-solar 2026 1 1     # lunar → Gregorian (when is this year's Spring Festival)
公历: 2026-02-17 (周二)

$ jdan lunar --to-solar 2025 6 1 --leap   # leap month (2025 has a leap 6th month)
$ jdan lunar 2026 --festivals        # list a year's lunar festivals (Spring Festival / Lantern / Dragon Boat / Qixi / Mid-Autumn / Double Ninth / New Year's Eve)
$ jdan lunar 2026-06-26 --json
```

**Correctness is guarded by real anchors + a full round-trip test**: the 2024/2025/2026 Spring Festivals, Mid-Autumn, Dragon Boat, and leap months (2025 leap-6, 2023 leap-2, 2020 leap-4) are each asserted, plus a `Gregorian → lunar → Gregorian` round-trip across the entire 1900–2100 range. Range is 1900–2100; out-of-range dates error. The ganzhi year boundary is the lunar new year (zodiac year). **Intentionally out of scope**: huangli almanac auspicious/inauspicious advice (no authoritative algorithm), the 24 solar terms (solar, a different computation), and any third-party lunar library (the embedded table suffices).

### `jdan readme`

Print the `README.md` content in a given directory (the current directory by default). The filename is case-insensitive, so `README.md` / `readme.md` / `Readme.md` etc. are all recognized.

```bash
jdan readme                      # print the current directory's README.md
jdan readme ./internal/cli       # relative path
jdan readme /path/to/project     # absolute path
jdan readme ~/code/myrepo        # supports ~ expansion
jdan readme --paging             # force the bat pager (page with space/enter, q to quit)
```

| Parameter | Description |
|------|------|
| `dir` | directory path (optional, defaults to the current directory) |
| `--paging` | when using bat, force paging (equivalent to `bat --paging=always`); no paging by default |

The rendering method is chosen by this priority:

1. If `bat` is in `PATH`, use `bat` (with syntax highlighting). It appends `--paging=never` by default for one-shot output; adding `--paging` appends `--paging=always` to enter a pager like less.
2. Otherwise if `cat` is present, use `cat` (`--paging` has no effect on `cat`).
3. If neither is available (such as a default Windows environment), read the file content directly and write it to standard output.

If the directory has no `README.md` in any case form, it errors out with a non-zero exit code.

### `jdan rand`

A family of random-generation subcommands. **All use `crypto/rand` (CSPRNG)**, never `math/rand`; character selection always goes through `crypto/rand.Int(charsetLen)`, never the `b[i] % len(charset)` mod-bias pattern (enforced by the `TestNoCharSelectionModulo` static gate).

9 subcommands, all accepting the shared flags `--count N` / `--json` / `--no-newline` (the last being mutually exclusive with `--count >1`):

```bash
jdan rand password                       # 1Password style: 20 chars + symbols + exclude ambiguous
jdan rand uuid                           # v4 by default
jdan rand uuid -V 7 -c 10                # 10 v7s (time-ordered)
jdan rand hex -l 32                      # 32 bytes → 64 hex chars
jdan rand base64 -l 32                   # standard base64
jdan rand base64url -l 32                # URL-safe base64 (no +/=)
jdan rand base32 -l 20                   # RFC 4648 base32
jdan rand alnum -l 12                    # alphanumeric (no per-class constraint)
jdan rand int 1 100                      # [1, 100] closed interval
jdan rand int -c 5 -- -10 10             # for negatives use -- as a separator, flags must come before --
jdan rand word                           # 6-word diceware passphrase (EFF 7776-word list)
jdan rand word -w 8 --sep "_"            # 8 words, underscore-separated
jdan rand hex --json -c 100              # 100 entries → JSON array (script-friendly)
jdan rand password --no-newline | pbcopy # a single entry with no newline, for piping
```

#### `jdan rand password`

| Parameter | Default | Description |
|------|------|------|
| `-l` / `--length` | `20` | password length |
| `--no-symbols` | `false` | alphanumeric only (still requires at least one of each class) |
| `--include-ambiguous` | `false` | don't exclude `I`/`l`/`1`/`O`/`0` |

Algorithm: **fixed positions + Fisher-Yates shuffle** (draw 1 character per class into a fixed position first, fill the rest with the full charset, then Fisher-Yates shuffle). Unbiased, and efficient even at the `-l 4` boundary.

`--no-symbols` is **different** from `jdan rand alnum`: the former still requires at least one of each of lower/upper/digit; the latter has no per-class constraint.

Entropy reference: the default 20 chars + symbols + exclude ambiguous ≈ 123 bits (charset of 71); `--no-symbols` ≈ 117 bits (charset of 57).

#### `jdan rand uuid`

| Parameter | Default | Description |
|------|------|------|
| `-V` / `--version` | `4` | UUID version (`4` or `7`) |

- **v4** = 122 random bits + version/variant markers. RFC 9562.
- **v7** = a 48-bit unix-millisecond timestamp + 74 random bits. Within the same millisecond `rand_a` provides roughly monotonic ordering, good for database indexes. RFC 9562.
- v1 (with MAC address) and v5 (SHA-1 namespace) are out of scope.

The UUID subcommand is **hand-written**, not pulling in the `github.com/google/uuid` dependency.

#### `jdan rand hex` / `base64` / `base64url` / `base32`

| Parameter | Default | Description |
|------|------|------|
| `-l` / `--length` | `32` | number of bytes (the encoded output is longer) |

- `hex` → outputs `2 × length` hex chars (`0-9a-f`)
- `base64` → standard base64 (with `+ / =` padding)
- `base64url` → URL-safe base64 (uses `- _`, no `=` padding, can go straight into a URL / JWT)
- `base32` → RFC 4648 uppercase `A-Z` + `2-7`. The Crockford variant isn't supported

#### `jdan rand alnum`

| Parameter | Default | Description |
|------|------|------|
| `-l` / `--length` | `20` | character length |
| `--include-ambiguous` | `false` | don't exclude `I`/`l`/`1`/`O`/`0` |

An alphanumeric string with **no per-class constraint** — `-l 1` is valid. Clearly distinct from `password --no-symbols`: the latter still requires at least one of each of lower/upper/digit.

#### `jdan rand int`

```bash
jdan rand int <min> <max>
```

| Parameter | Default | Description |
|------|------|------|
| `min` `max` | — | required, `cobra.ExactArgs(2)` |
| `-c` / `--count` | `1` | how many to generate |
| `-j` / `--json` | `false` | a JSON array of **integers** (not strings) |

A closed interval `[min, max]`, supporting negatives / crossing zero / `min == max`. For negatives use `--` as a separator, and flags must come **before** the `--`:

```bash
jdan rand int -c 5 -- -10 10   # ✓ correct
jdan rand int -- -10 10 -c 5   # ✗ wrong (-c 5 is treated as positional)
```

`--no-newline` isn't supported (integer + newline is the standard stdout format).

#### `jdan rand word`

| Parameter | Default | Description |
|------|------|------|
| `-w` / `--words` | `6` | the number of words per passphrase |
| `--sep` | `-` | the separator between words (an empty string is valid, producing an unsplittable string) |

Draws words from the **EFF Large Wordlist** (7776 words, CC-BY 3.0, embedded in the binary via `go:embed`, SHA256-verified at `init()`). 12.9 bits of entropy per word; the default 6 words is about 77.5 bits of entropy (an alnum password over 12 chars ≈ 71 bits).

Note that **`--words` is the number of words per passphrase; `--count` is the number of passphrases**:

```bash
jdan rand word                         # 1 6-word passphrase
jdan rand word -w 8                    # 1 8-word passphrase
jdan rand word -c 5                    # 5 6-word passphrases (one per line)
jdan rand word -w 8 -c 5 --json        # 5 8-word passphrases → JSON array
```

> Currently macOS + Linux only (following jdan's status quo).

### `jdan uuid`

Inspect a UUID: version, variant, embedded v1/v7 timestamp, bytes, URN form, nil/max. Generation lives in `jdan rand uuid`; this command does the **parsing** (`jdan uuid new` is a thin wrapper reusing the same implementation, zero logic duplication). Zero new dependencies.

Detailed technical docs: [docs/jdan-uuid.md](docs/jdan-uuid.md)

```bash
$ jdan uuid 0190a1b2-c3d4-7e5f-8a9b-1c2d3e4f5a6b
canonical: 0190a1b2-c3d4-7e5f-8a9b-1c2d3e4f5a6b
version:   7 (时间排序)
variant:   RFC 4122
time:      2026-06-26 14:00:00.000 UTC
bytes:     01 90 a1 b2 c3 d4 7e 5f 8a 9b 1c 2d 3e 4f 5a 6b
urn:       urn:uuid:0190a1b2-c3d4-7e5f-8a9b-1c2d3e4f5a6b

# input-tolerant (urn: prefix / {braces} / no hyphens / case) + stdin + JSON
$ echo "$U" | jdan uuid --json
$ jdan uuid new --v7 -n 3      # generate (reuses jdan rand uuid)
```

v7/v1 timestamps are decoded automatically; nil (all zero) / max (all F) are flagged. Invalid UUIDs get a clear error.

### `jdan fake`

Generate realistic-looking structured fake values, for building test fixtures, seeding a database, writing examples. Zero new dependencies (built-in word lists). Complements `jdan rand` (meaningless random strings) — `fake` gives you realistic-looking structured fake values.

Detailed technical docs: [docs/jdan-fake.md](docs/jdan-fake.md)

**Types**: name / email / uuid / sentence / word / int / date / ip

```bash
$ jdan fake name
Alice Chen

$ jdan fake email -n 3
bob.patel@example.net
amy.wong@test.org
leo.kim@demo.net

# --seed for reproducibility (build stable fixtures)
$ jdan fake name --seed 42 -n 2
Zack Walker
Cleo King

$ jdan fake int --min 1 --max 6      # dice
$ jdan fake uuid --json -n 5         # JSON array

# no type + --json → composite records
$ jdan fake --json -n 2
[
  {"name":"Bob Patel","email":"bob.patel@example.net","age":74,"ip":"198.51.100.134"},
  {"name":"Zack Thomas","email":"zack.thomas@example.org","age":33,"ip":"203.0.113.175"}
]
```

`--seed N` switches to a deterministic sequence (same seed, same output; `date` uses a fixed window and doesn't depend on the current date); without it, it uses `crypto/rand` for true randomness. IPs only use the RFC 5737 documentation-reserved ranges and emails use RFC 2606 example domains, safely avoiding collisions with real resources. `--list` lists the types.

### `jdan git summary`

The repo at a glance: total commit count, branches, tags, age, contributor leaderboard, the most-changed files (hotspots). Read-only. jdan's first git command, shelling out to `git` underneath (**zero new Go dependencies**, just needs git in the environment).

Detailed technical docs: [docs/jdan-git-summary.md](docs/jdan-git-summary.md)

```bash
$ jdan git summary
仓库: jdan
commit: 77   分支: 5   tag: 2
年龄: 2026-03-21 起 (约 2 个月)

贡献者 Top 5:
  xunull  77 (100.0%)

改动最多的文件 (hotspots) Top 5:
  README.md            40 次
  go.mod                9 次
  internal/cli/dns.go   4 次

# specify a repo / control the leaderboard length / structured output
$ jdan git summary /path/repo
$ jdan git summary --top 10
$ jdan git summary --json
```

Age uses the span "first commit → last commit" (independent of the system clock, reproducible). Non-git repos / empty repos / a missing git all get a clear error.

### `jdan git changelog`

Generate a changelog from the latest tag to HEAD, grouped by Conventional Commits (feat→Features / fix→Bug Fixes / …, breaking changes pulled out on their own). Fits the `feat()/fix()` commit style, one command for release notes before a version bump. Outputs markdown by default, ready to redirect. Shells out to `git` underneath (**zero new dependencies**).

Detailed technical docs: [docs/jdan-git-changelog.md](docs/jdan-git-changelog.md)

```bash
$ jdan git changelog
## 未发布 (自 v0.5.2)

### ⚠ Breaking Changes
- (api) drop the v1 endpoint

### Features
- (json) add json merge for deep-merging
- (ping) add jdan ping with --dns

### Bug Fixes
- (figlet) block font UTF-8 panic

# specify a range / structured output
$ jdan git changelog --from v0.4.0 --to v0.5.0
$ jdan git changelog > RELEASE.md
$ jdan git changelog --json
```

The range defaults to "latest tag → HEAD" (whole history if there are no tags); merge commits are skipped by default; non-conforming subjects land in Other (nothing dropped). Non-git repos / invalid refs get a clear error.

### `jdan toc`

Generate a table of contents from Markdown headings, with anchors that match **GitHub's rendering rules**, so you can paste it straight back into a README. Zero new dependencies (pure stdlib).

Detailed technical docs: [docs/jdan-toc.md](docs/jdan-toc.md)

```bash
$ jdan toc README.md
- [安装](#安装)
  - [方式 1：下载预编译二进制（推荐）](#方式-1下载预编译二进制推荐)
- [命令](#命令)
  - [`jdan qr`](#jdan-qr)
  - [`jdan figlet`](#jdan-figlet)

# limit levels / write back in place
$ jdan toc README.md --min 2 --max 3
$ jdan toc README.md --inplace   # replaces between the <!-- toc --> markers
```

The anchor algorithm (lowercase + strip punctuation like backticks and `#` + spaces to hyphens + `-1`/`-2` suffix on duplicate headings) matches GitHub (verified against this very README, anchor for anchor). Defaults to starting at h2 (skips the document title). A `#` inside a code fence is never mistaken for a heading. `--inplace` errors if the markers are missing and is idempotent (safe to re-run).

### `jdan obsidian install-claudian`

Download the [Claudian](https://github.com/YishenTu/claudian) plugin files from the latest GitHub Release and install them into a given Obsidian Vault.

```bash
jdan obsidian install-claudian ./my-vault       # install into a given vault directory
jdan obsidian install-claudian                  # install into the current directory
jdan obsidian install-claudian ~/Documents/vault --force  # overwrite an already-installed version
```

| Parameter | Description |
|------|------|
| `vault-path` | the Vault directory path (optional, defaults to the current directory) |
| `--force` / `-f` | force overwrite if the plugin is already installed |

After a successful install it creates `main.js`, `manifest.json`, `styles.css` under `{vault}/.obsidian/plugins/claudian/`; then just enable it in Obsidian's Settings → Community plugins.

## Global flags

Every subcommand accepts:

| Parameter | Description |
|------|------|
| `--config` | config file path (optional; loaded via viper, each subcommand decides whether to consume it) |
| `-h` / `--help` | subcommand help |

## Development

```bash
# unit tests (integration tests that need the network are skipped by default)
go test ./...

# integration tests (actually hit DNS / DoH; not run by default in CI)
go test -tags integration ./internal/dnslookup/... ./internal/dnstrace/...

# build (if a go.work in a parent directory interferes)
GOWORK=off go build -o jdan .
```

Design docs are under `docs/brainstorms/` and `docs/plans/`, ordered by time, with each new subcommand typically corresponding to a brainstorm + plan pair.
