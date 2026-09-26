// 多彩种回测：数字型投影复用 kill6 引擎回测；号码型冷热/遗漏杀号回测。
// 与福彩3D/双色球回测同构，严格按期号回溯、无未来信息，如实对照随机基线。
package backtest

import (
	"fmt"
	"math"

	"fc3d-kill6/data"
	"fc3d-kill6/engine/num"
)

// ── 数字型（排列3/排列5/七星彩）：末三位投影 → 复用 kill6 回测 ────

// DigitBacktest 取数字型彩种每期末三位投影为 (百,十,个)，完整复用
// 福彩3D 的 RunAll / MultiWindow / WalkForward（引擎、基线、显著性口径一致）。
func DigitBacktest(draws []data.NumDraw) (*Result, map[string]WindowStats, []WFWindow) {
	proj := data.Project3(draws)
	return RunAll(proj), MultiWindow(proj, []int{100, 200, 300, 500}), WalkForward(proj, []int{100, 200, 300, 500})
}

// ── 号码型（大乐透/七乐彩/快乐8）：双区杀号回测 ────

// NumCfg 号码型回测配置：A 区（主区）+ 可选 B 区（副区）。
type NumCfg struct {
	MaxA, PickA, KillA int // A 区号码空间 1..MaxA，每期开 PickA 个，杀 KillA 个
	MaxB, PickB, KillB int // B 区；MaxB==0 表示无 B 区（如快乐8）
	Window             int
	Strategy           num.Strategy
}

// NumCfgFor 轻杀配置（高命中口径）：杀号数压到 1 个，随机基线 75%~86%。
// 命中率由杀号数量结构性决定（组合数学），策略本身无法超越随机——
// 轻杀=高命中弱过滤，深杀=低命中强过滤，页面双口径同时呈现。
// 策略取全量回测相对最稳者（tools/num_probe 探针，差异均在噪声范围内，如实披露）：
// 大乐透=冷号法(+0.97pp)、七乐彩=热号法(+1.21pp)、快乐8=遗漏法(+1.10pp)。
func NumCfgFor(g data.GameDef) (NumCfg, bool) {
	switch g.Key {
	case "dlt":
		return NumCfg{MaxA: 35, PickA: 5, KillA: 1, MaxB: 12, PickB: 2, KillB: 1, Window: 50, Strategy: num.StrategyCold}, true
	case "qlc":
		return NumCfg{MaxA: 30, PickA: 7, KillA: 1, MaxB: 30, PickB: 1, KillB: 0, Window: 50, Strategy: num.StrategyHot}, true
	case "kl8":
		return NumCfg{MaxA: 80, PickA: 20, KillA: 1, MaxB: 0, Window: 50, Strategy: num.StrategyMiss}, true
	}
	return NumCfg{}, false
}

// NumCfgDeepFor 深杀配置（强过滤口径）：随机基线 22%~37%，供缩小选号范围用。
func NumCfgDeepFor(g data.GameDef) (NumCfg, bool) {
	switch g.Key {
	case "dlt":
		return NumCfg{MaxA: 35, PickA: 5, KillA: 5, MaxB: 12, PickB: 2, KillB: 1, Window: 50, Strategy: num.StrategyCold}, true
	case "qlc":
		return NumCfg{MaxA: 30, PickA: 7, KillA: 5, MaxB: 30, PickB: 1, KillB: 2, Window: 50, Strategy: num.StrategyHot}, true
	case "kl8":
		return NumCfg{MaxA: 80, PickA: 20, KillA: 5, MaxB: 0, Window: 50, Strategy: num.StrategyMiss}, true
	}
	return NumCfg{}, false
}

// NumRow 号码型回测明细中的一行
type NumRow struct {
	Issue string
	Date  string
	Open  string
	KillA []int
	KillB []int
	AOK   bool // 杀 A 区全避开
	BOK   bool // 杀 B 区全避开（无 B 区恒 true）
	AllOK bool
}

// NumMeta 号码型回测元数据
type NumMeta struct {
	Game                                 data.GameDef
	Total                                int
	LatestIssue                          string
	LatestDate                           string
	NextIssue                            string
	Strategy                             string
	Window                               int
	KillA, KillB                         []int
	KillAN, KillBN                       int
	APct, BPct, AllPct                   float64 // 全量回测
	BaseA, BaseB, BaseAll                float64
	RecentAPct, RecentBPct, RecentAllPct float64  // 最近 100 期
	Rows                                 []NumRow // 尾部 100 期明细（最新在前）
	WF                                   []WFWindow
}

// NumBacktest 号码型杀号回测：从第 window 期开始，每期用前 window 期数据
// 按策略算杀号，对照当期开奖。输出全量/近 100 期命中率、随机基线与 walk-forward。
func NumBacktest(draws []data.NumDraw, g data.GameDef, cfg NumCfg) NumResult {
	total := len(draws)
	ahit, bhit, allhit, n := 0, 0, 0, 0
	ahit2, bhit2, allhit2, n2 := 0, 0, 0, 0
	all := make([]NumRow, 0, total-cfg.Window)
	hasB := cfg.MaxB > 0 && cfg.KillB > 0
	for t := cfg.Window; t < total; t++ {
		win := draws[t-cfg.Window : t]
		ka := num.KillNums(win, frontN(cfg.PickA), cfg.MaxA, cfg.KillA, 0, cfg.Strategy)
		var kb []int
		if hasB {
			kb = num.KillNums(win, backN(cfg.PickA), cfg.MaxB, cfg.KillB, 0, cfg.Strategy)
		}
		d := draws[t]
		aOK, bOK := true, true
		for _, r := range frontN(cfg.PickA)(d) {
			if containsInt(ka, r) {
				aOK = false
				break
			}
		}
		if hasB {
			for _, r := range backN(cfg.PickA)(d) {
				if containsInt(kb, r) {
					bOK = false
					break
				}
			}
		}
		if aOK {
			ahit++
		}
		if bOK {
			bhit++
		}
		if aOK && bOK {
			allhit++
		}
		if t >= total-100 {
			n2++
			if aOK {
				ahit2++
			}
			if bOK {
				bhit2++
			}
			if aOK && bOK {
				allhit2++
			}
		}
		all = append(all, NumRow{
			Issue: d.Issue, Date: d.Date, Open: d.OpenString(g),
			KillA: ka, KillB: kb, AOK: aOK, BOK: bOK, AllOK: aOK && bOK,
		})
		n++
	}

	// 本期杀号（用最近 window 期）
	lastWin := draws[total-cfg.Window:]
	ka := num.KillNums(lastWin, frontN(cfg.PickA), cfg.MaxA, cfg.KillA, 0, cfg.Strategy)
	var kb []int
	if hasB {
		kb = num.KillNums(lastWin, backN(cfg.PickA), cfg.MaxB, cfg.KillB, 0, cfg.Strategy)
	}

	rows := all
	if len(rows) > 100 {
		rows = rows[len(rows)-100:]
	}
	rev := make([]NumRow, len(rows))
	for i := range rows {
		rev[len(rows)-1-i] = rows[i]
	}

	baseA := combF(cfg.MaxA-cfg.KillA, cfg.PickA) / combF(cfg.MaxA, cfg.PickA) * 100
	baseB := 100.0
	if hasB {
		baseB = combF(cfg.MaxB-cfg.KillB, cfg.PickB) / combF(cfg.MaxB, cfg.PickB) * 100
	}
	meta := NumMeta{
		Game: g, Total: total,
		LatestIssue: draws[total-1].Issue, LatestDate: draws[total-1].Date,
		Strategy: cfg.Strategy.StrName(), Window: cfg.Window,
		KillA: ka, KillB: kb, KillAN: cfg.KillA, KillBN: cfg.KillB,
		APct: pct(ahit, n), BPct: pct(bhit, n), AllPct: pct(allhit, n),
		BaseA: baseA, BaseB: baseB, BaseAll: baseA * baseB / 100,
		RecentAPct: pct(ahit2, n2), RecentBPct: pct(bhit2, n2), RecentAllPct: pct(allhit2, n2),
		Rows: rev,
		WF:   numWalkForward(draws, g, cfg, []int{100, 200, 500}),
	}
	return NumResult{Meta: meta}
}

// NumResult 号码型回测输出
type NumResult struct {
	Meta NumMeta
}

// frontN 抽取前 pick 个号码（A 区）
func frontN(pick int) func(data.NumDraw) []int {
	return func(d data.NumDraw) []int {
		if len(d.Nums) < pick {
			return d.Nums
		}
		return d.Nums[:pick]
	}
}

// backN 抽取 pick 之后的号码（B 区）
func backN(pick int) func(data.NumDraw) []int {
	return func(d data.NumDraw) []int {
		if len(d.Nums) <= pick {
			return nil
		}
		return d.Nums[pick:]
	}
}

// numWalkForward 号码型多窗口 walk-forward：全中率 vs 基线二项检验。
func numWalkForward(draws []data.NumDraw, g data.GameDef, cfg NumCfg, windows []int) []WFWindow {
	total := len(draws)
	base := (combF(cfg.MaxA-cfg.KillA, cfg.PickA) / combF(cfg.MaxA, cfg.PickA)) *
		(func() float64 {
			if cfg.MaxB == 0 {
				return 1
			}
			return combF(cfg.MaxB-cfg.KillB, cfg.PickB) / combF(cfg.MaxB, cfg.PickB)
		})()
	out := []WFWindow{}
	for _, w := range windows {
		if total <= w+cfg.Window {
			continue
		}
		start := total - w
		allhit, n := 0, 0
		for t := start; t < total; t++ {
			if t < cfg.Window {
				continue
			}
			win := draws[t-cfg.Window : t]
			ka := num.KillNums(win, frontN(cfg.PickA), cfg.MaxA, cfg.KillA, 0, cfg.Strategy)
			d := draws[t]
			ok := true
			for _, r := range frontN(cfg.PickA)(d) {
				if containsInt(ka, r) {
					ok = false
					break
				}
			}
			if ok && cfg.MaxB > 0 {
				kb := num.KillNums(win, backN(cfg.PickA), cfg.MaxB, cfg.KillB, 0, cfg.Strategy)
				for _, r := range backN(cfg.PickA)(d) {
					if containsInt(kb, r) {
						ok = false
						break
					}
				}
			}
			if ok {
				allhit++
			}
			n++
		}
		if n == 0 {
			continue
		}
		pctVal := pct(allhit, n)
		se := math.Sqrt(base * (1 - base) / float64(n))
		z := 0.0
		if se > 0 {
			z = (pctVal/100 - base) / se
		}
		out = append(out, WFWindow{
			Label: fmt.Sprintf("%d期", w), N: n, All6: allhit, All6Pct: pctVal,
			BeatPP: pctVal - base*100, Z: z, PVal: normP(z),
		})
	}
	return out
}
