package tradx

import "fmt"

// buildChain 按目标 config 组装 conversion 链。分组严格对照真 config
// （data/config/{s2t,t2s,s2tw,s2twp,s2hk}.json）与源码 group policy：
//
//	s2t   : [ conv( SC{ STPhrases, STCharacters } ) ]              # U{单词典}=该词典
//	t2s   : [ conv( SC{ TSPhrases, TSCharacters } ) ]              # 略过 build-generated TSCharactersExt
//	s2tw  : [ s2t链, conv(SC{ TWVariantsPhrases, TWVariants }) ]
//	s2twp : [ s2t链, conv(SC{ TWPhrases, TWVariantsPhrases, TWVariants }) ]
//	s2hk  : [ s2t链, conv(SC{ HKVariantsPhrases, HKVariants }) ]
//
// 缺失的 STPhrases_GeneratedFromRegionalPhrases 不含（见包文档缺口说明）。
func buildChain(target string) ([]matcher, error) {
	dd, err := getDicts()
	if err != nil {
		return nil, err
	}
	// sc 构造一个 short_circuit 组，成员按给定名字顺序取词典。
	sc := func(names ...string) *group {
		ms := make([]matcher, len(names))
		for i, n := range names {
			ms[i] = dd[n]
		}
		return &group{shortCircuit: true, members: ms}
	}

	s2t := sc("STPhrases", "STCharacters")

	switch target {
	case ToT:
		return []matcher{s2t}, nil
	case ToS:
		return []matcher{sc("TSPhrases", "TSCharacters")}, nil
	case ToTW:
		return []matcher{s2t, sc("TWVariantsPhrases", "TWVariants")}, nil
	case ToTWP:
		return []matcher{s2t, sc("TWPhrases", "TWVariantsPhrases", "TWVariants")}, nil
	case ToHK:
		return []matcher{s2t, sc("HKVariantsPhrases", "HKVariants")}, nil
	}
	return nil, fmt.Errorf("未知转换目标 %q（可选 t/tw/twp/hk/s）", target)
}
