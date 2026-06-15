package dotenv

import "fmt"

// Get 返回指定 key 的 value（重复 key 取最后一个，跟 shell 加载语义一致）。
// key 不存在返回错误。
func Get(f *File, key string) (string, error) {
	found := false
	var val string
	for _, e := range f.Entries {
		if e.HasEquals && e.Key == key {
			val = e.Value
			found = true
		}
	}
	if !found {
		return "", fmt.Errorf("key %q not found", key)
	}
	return val, nil
}
