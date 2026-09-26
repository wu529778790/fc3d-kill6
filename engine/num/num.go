// Package num 实现号码型彩种（大乐透/七乐彩/快乐8）的冷热/遗漏/杀号统计引擎。
// 与 engine/ssq 同构，但作用于通用 NumDraw 与可配置号码空间（1..maxN）。
// 纯函数、可回测；策略与双色球一致：冷号法/热号法/遗漏法，由回测决定采用哪种。
package num

import (
	"sort"

	"fc3d-kill6/data"
)

// Strategy 杀号策略（与 engine/ssq.Strategy 语义一致）
type Strategy int

const (
	StrategyCold Strategy = iota // 最冷：近 window 期出现次数最少的号码
	StrategyHot                  // 最热：近 window 期出现次数最多的号码
	StrategyMiss                 // 遗漏：距离上次开出最久（期数最长）
)

// StrName 策略中文名
func (s Strategy) StrName() string {
	switch s {
	case StrategyCold:
		return "冷号法"
	case StrategyHot:
		return "热号法"
	case StrategyMiss:
		return "遗漏法"
	}
	return "未知"
}

// NumFreq 号码频率
type NumFreq struct {
	Num  int
	Freq int
}

// NumMiss 号码遗漏
type NumMiss struct {
	Num  int
	Miss int
}

// Freq 统计窗口内各号码（1..maxN）出现次数（升序返回，号码 1 在前）。
func Freq(draws []data.NumDraw, window, maxN int) []NumFreq {
	cnt := make([]int, maxN+1)
	start := 0
	if window > 0 && window < len(draws) {
		start = len(draws) - window
	}
	for _, d := range draws[start:] {
		for _, n := range d.Nums {
			if n >= 1 && n <= maxN {
				cnt[n]++
			}
		}
	}
	out := make([]NumFreq, maxN)
	for n := 1; n <= maxN; n++ {
		out[n-1] = NumFreq{Num: n, Freq: cnt[n]}
	}
	return out
}

// Miss 统计窗口内各号码距窗口末尾的遗漏期数（升序返回）。
func Miss(draws []data.NumDraw, window, maxN int) []NumMiss {
	last := make([]int, maxN+1)
	start := 0
	if window > 0 && window < len(draws) {
		start = len(draws) - window
	}
	for i := start; i < len(draws); i++ {
		for _, n := range draws[i].Nums {
			if n >= 1 && n <= maxN {
				last[n] = i
			}
		}
	}
	end := len(draws) - 1
	out := make([]NumMiss, maxN)
	for n := 1; n <= maxN; n++ {
		if last[n] == 0 && !appeared(draws, start, n) {
			out[n-1] = NumMiss{Num: n, Miss: end - start + 1} // 窗口内从未出现，视为整窗遗漏
		} else {
			out[n-1] = NumMiss{Num: n, Miss: end - last[n]}
		}
	}
	return out
}

func appeared(draws []data.NumDraw, start, n int) bool {
	for i := start; i < len(draws); i++ {
		for _, v := range draws[i].Nums {
			if v == n {
				return true
			}
		}
	}
	return false
}

// pickLowest 选 score 最小的 n 个号码（并列按号码小优先）
func pickLowest(score []NumFreq, n int) []int {
	ps := make([]NumFreq, len(score))
	copy(ps, score)
	sort.SliceStable(ps, func(a, b int) bool {
		if ps[a].Freq != ps[b].Freq {
			return ps[a].Freq < ps[b].Freq
		}
		return ps[a].Num < ps[b].Num
	})
	out := make([]int, 0, n)
	for i := 0; i < n && i < len(ps); i++ {
		out = append(out, ps[i].Num)
	}
	return out
}

// pickHighest 选 score 最大的 n 个号码（并列按号码大优先）
func pickHighest(score []NumFreq, n int) []int {
	ps := make([]NumFreq, len(score))
	copy(ps, score)
	sort.SliceStable(ps, func(a, b int) bool {
		if ps[a].Freq != ps[b].Freq {
			return ps[a].Freq > ps[b].Freq
		}
		return ps[a].Num > ps[b].Num
	})
	out := make([]int, 0, n)
	for i := 0; i < n && i < len(ps); i++ {
		out = append(out, ps[i].Num)
	}
	return out
}

// KillNums 基于最近 window 期数据的指定号码区（numsOf 回调抽取该区的号码），
// 按策略选出 n 个最不可能开出的号码。draws 按时间正序（最新在末尾）。
// window<=0 或超界时用全部数据。
func KillNums(draws []data.NumDraw, numsOf func(data.NumDraw) []int, maxN, n, window int, s Strategy) []int {
	sub := make([]data.NumDraw, len(draws))
	for i, d := range draws {
		sub[i] = data.NumDraw{Issue: d.Issue, Date: d.Date, Nums: numsOf(d)}
	}
	switch s {
	case StrategyHot:
		return pickHighest(Freq(sub, window, maxN), n)
	case StrategyMiss:
		m := Miss(sub, window, maxN)
		fs := make([]NumFreq, len(m))
		for i, v := range m {
			fs[i] = NumFreq{Num: v.Num, Freq: v.Miss}
		}
		return pickLowest(fs, n)
	default:
		return pickLowest(Freq(sub, window, maxN), n)
	}
}
