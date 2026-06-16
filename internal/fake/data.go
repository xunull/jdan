package fake

// 内置词库。全部是示例/保留值，不对应真实个人，仅供测试。

var firstNames = []string{
	"Alice", "Bob", "Carol", "Dave", "Eve", "Frank", "Grace", "Heidi",
	"Ivan", "Judy", "Karl", "Liam", "Mona", "Nina", "Oscar", "Peggy",
	"Quinn", "Ruth", "Sam", "Trent", "Uma", "Victor", "Wendy", "Xena",
	"Yara", "Zack", "Amy", "Ben", "Cleo", "Dan", "Ella", "Finn",
	"Gina", "Hugo", "Iris", "Jack", "Kira", "Leo", "Maya", "Noah",
}

var lastNames = []string{
	"Smith", "Johnson", "Lee", "Brown", "Garcia", "Wong", "Chen", "Patel",
	"Khan", "Singh", "Lopez", "Kim", "Wang", "Nguyen", "Davis", "Miller",
	"Wilson", "Moore", "Taylor", "Anderson", "Thomas", "Jackson", "White", "Harris",
	"Martin", "Clark", "Lewis", "Walker", "Hall", "Young", "King", "Wright",
	"Hill", "Green", "Adams", "Baker", "Nelson", "Carter", "Ng", "Yang",
}

var loremWords = []string{
	"lorem", "ipsum", "dolor", "sit", "amet", "consectetur", "adipiscing", "elit",
	"sed", "do", "eiusmod", "tempor", "incididunt", "ut", "labore", "et",
	"dolore", "magna", "aliqua", "enim", "ad", "minim", "veniam", "quis",
	"nostrud", "exercitation", "ullamco", "laboris", "nisi", "aliquip", "ex", "ea",
	"commodo", "consequat", "duis", "aute", "irure", "in", "reprehenderit", "voluptate",
	"velit", "esse", "cillum", "fugiat", "nulla", "pariatur", "excepteur", "sint",
	"occaecat", "cupidatat", "non", "proident", "culpa", "qui", "officia", "deserunt",
	"mollit", "anim", "id", "est", "laborum",
}

// 邮箱域名：全部是 RFC 2606/6761 保留的示例域名，不撞真实邮箱。
var domains = []string{
	"example.com", "example.org", "example.net", "test.org", "demo.net",
}

// docIPBlocks 是 RFC 5737 文档保留段的 /24 前缀（前三段），随机只动最后一段。
// 这些地址不可路由，不会撞真实主机，安全。
var docIPBlocks = []string{
	"192.0.2", "198.51.100", "203.0.113",
}
