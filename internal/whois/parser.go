package whois

import (
	"strings"
	"time"
)

// Parsed 是 WHOIS response best-effort 解析出的结构化字段。不同 registrar
// schema 不一致，所有字段都是 best-effort：缺失时零值；list 字段 nil 表示
// 该 server 未输出该字段（原文里也没有）。
//
// 设计选择：保留原文（Result.RawText 永远在）做兜底，parser 失败不影响
// 命令可用性。
type Parsed struct {
	// 域名相关
	DomainName        string    `json:"domain_name,omitempty"`
	Registrar         string    `json:"registrar,omitempty"`
	CreationDate      time.Time `json:"creation_date,omitzero"`
	UpdatedDate       time.Time `json:"updated_date,omitzero"`
	ExpiryDate        time.Time `json:"expiry_date,omitzero"`
	Status            []string  `json:"status,omitempty"`
	Nameservers       []string  `json:"nameservers,omitempty"`
	RegistrantCountry string    `json:"registrant_country,omitempty"`
	DNSSEC            string    `json:"dnssec,omitempty"`
	RegistryDomainID  string    `json:"registry_domain_id,omitempty"`

	// IP 相关
	NetRange   string    `json:"netrange,omitempty"`
	NetName    string    `json:"netname,omitempty"`
	OrgName    string    `json:"org_name,omitempty"`
	Country    string    `json:"country,omitempty"`
	AbuseEmail string    `json:"abuse_email,omitempty"`
	RegDate    time.Time `json:"reg_date,omitzero"`
}

// IsEmpty 判断 parser 是否一个字段都没抓到。CLI 据此决定 fallback 到 raw。
func (p *Parsed) IsEmpty() bool {
	if p == nil {
		return true
	}
	return p.DomainName == "" && p.Registrar == "" &&
		p.CreationDate.IsZero() && p.UpdatedDate.IsZero() && p.ExpiryDate.IsZero() &&
		len(p.Status) == 0 && len(p.Nameservers) == 0 &&
		p.RegistrantCountry == "" && p.DNSSEC == "" && p.RegistryDomainID == "" &&
		p.NetRange == "" && p.NetName == "" && p.OrgName == "" &&
		p.Country == "" && p.AbuseEmail == "" && p.RegDate.IsZero()
}

// aliasGroup 把多个 schema 的别名映射到同一逻辑字段。
// 例：Verisign 用 "Registry Expiry Date"，Identity Digital 也支持
// "Registrar Registration Expiration Date"，DENIC 用 "Expires on" —— 都映射到 ExpiryDate。
type aliasGroup struct {
	field string
	keys  []string
	multi bool // true 表示同一 key 出现多次都要收（Status / Name Server）
}

var domainAliases = []aliasGroup{
	{field: "domain_name", keys: []string{"domain name", "domain"}},
	{field: "registrar", keys: []string{"registrar", "sponsoring registrar"}},
	{field: "creation", keys: []string{"creation date", "created on", "created", "registered", "registration time"}},
	{field: "updated", keys: []string{"updated date", "last updated", "last modified", "modified"}},
	{field: "expiry", keys: []string{"registry expiry date", "registrar registration expiration date", "expiration date", "expiry date", "expires on", "expires", "expire"}},
	{field: "status", keys: []string{"domain status", "status"}, multi: true},
	{field: "nameserver", keys: []string{"name server", "nserver", "nameserver"}, multi: true},
	{field: "registrant_country", keys: []string{"registrant country"}},
	{field: "dnssec", keys: []string{"dnssec"}},
	{field: "registry_domain_id", keys: []string{"registry domain id"}},
}

var ipAliases = []aliasGroup{
	{field: "netrange", keys: []string{"netrange", "inetnum", "inet6num", "cidr"}},
	{field: "netname", keys: []string{"netname"}},
	{field: "org_name", keys: []string{"orgname", "org-name", "owner", "responsible", "org"}},
	{field: "country", keys: []string{"country"}},
	{field: "abuse_email", keys: []string{"orgabuseemail", "abuse-mailbox", "abuse contact email", "abuse-c"}},
	{field: "reg_date", keys: []string{"regdate", "created", "registered"}},
}

// scan 把 raw WHOIS 文本切成 key→[]value（按出现顺序）。
//   - 跳过空行 / "%" / "#" 注释
//   - 第一个 ":" 切分 key/value
//   - key 小写 + trim；value 仅 trim
func scan(raw string) map[string][]string {
	out := map[string][]string{}
	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "%") || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		val := strings.TrimSpace(line[idx+1:])
		if val == "" {
			continue
		}
		out[key] = append(out[key], val)
	}
	return out
}

func findValues(kv map[string][]string, keys []string) []string {
	for _, k := range keys {
		if v, ok := kv[k]; ok {
			return v
		}
	}
	return nil
}

// ParseDomain 提取域名 WHOIS 的常见字段。
func ParseDomain(raw string) *Parsed {
	kv := scan(raw)
	p := &Parsed{}
	for _, g := range domainAliases {
		vals := findValues(kv, g.keys)
		if len(vals) == 0 {
			continue
		}
		switch g.field {
		case "domain_name":
			p.DomainName = vals[0]
		case "registrar":
			p.Registrar = vals[0]
		case "creation":
			p.CreationDate = parseWhoisTime(vals[0])
		case "updated":
			p.UpdatedDate = parseWhoisTime(vals[0])
		case "expiry":
			p.ExpiryDate = parseWhoisTime(vals[0])
		case "status":
			for _, v := range vals {
				p.Status = append(p.Status, cleanStatusValue(v))
			}
		case "nameserver":
			for _, v := range vals {
				p.Nameservers = append(p.Nameservers, strings.ToLower(firstWord(v)))
			}
		case "registrant_country":
			p.RegistrantCountry = vals[0]
		case "dnssec":
			p.DNSSEC = vals[0]
		case "registry_domain_id":
			p.RegistryDomainID = vals[0]
		}
	}
	return p
}

// ParseIP 提取 IP WHOIS 的常见字段（ARIN/RIPE/APNIC 等）。
func ParseIP(raw string) *Parsed {
	kv := scan(raw)
	p := &Parsed{}
	for _, g := range ipAliases {
		vals := findValues(kv, g.keys)
		if len(vals) == 0 {
			continue
		}
		switch g.field {
		case "netrange":
			p.NetRange = vals[0]
		case "netname":
			p.NetName = vals[0]
		case "org_name":
			p.OrgName = vals[0]
		case "country":
			p.Country = vals[0]
		case "abuse_email":
			p.AbuseEmail = vals[0]
		case "reg_date":
			p.RegDate = parseWhoisTime(vals[0])
		}
	}
	return p
}

// timeLayouts 是 WHOIS server 输出日期的常见格式，按"严格→宽松"顺序尝试。
var timeLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05Z",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"02-Jan-2006",
	"2006.01.02",
}

// parseWhoisTime 试 timeLayouts 中第一个能解析的格式。
// 解析失败返回零值（time.Time{}），不报错 —— best-effort 语义。
func parseWhoisTime(s string) time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	// 某些日期带额外文字（"2026-08-13 (UTC)"）—— 取第一个 token 再试
	if first := firstWord(s); first != s {
		for _, layout := range timeLayouts {
			if t, err := time.Parse(layout, first); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

// cleanStatusValue 把 "clientDeleteProhibited https://..." 剥成 "clientDeleteProhibited"
// （Verisign 风格 status 行尾带 EPP 文档 URL，无信息量）。
func cleanStatusValue(s string) string {
	return firstWord(s)
}

func firstWord(s string) string {
	s = strings.TrimSpace(s)
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			return s[:i]
		}
	}
	return s
}
