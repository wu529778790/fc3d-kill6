// num_probe — 一次性探针：对比号码型各彩种 冷/热/遗漏 三策略全量回测命中率。
// 用于确认策略间差异是否显著，主程序策略选择提供依据。用完可删。
package main

import (
	"fmt"

	"fc3d-kill6/backtest"
	"fc3d-kill6/data"
	"fc3d-kill6/engine/num"
)

func main() {
	for _, g := range data.MultiGames {
		if g.Digit {
			continue
		}
		draws, err := data.LoadNumCSV(g.Key+"-history.csv", g)
		if err != nil || len(draws) < 200 {
			fmt.Printf("%s: 数据不足 %v\n", g.Key, err)
			continue
		}
		cfg, _ := backtest.NumCfgFor(g)
		fmt.Printf("=== %s (轻杀%d, 基线约%.1f%%) ===\n", g.Name, cfg.KillA, func() float64 {
			c := cfg
			return 100 * float64(c.MaxA-c.KillA) / float64(c.MaxA)
		}())
		for _, s := range []num.Strategy{num.StrategyCold, num.StrategyHot, num.StrategyMiss} {
			c := cfg
			c.Strategy = s
			m := backtest.NumBacktest(draws, g, c).Meta
			fmt.Printf("  %s: 全避开 %.2f%% (基线 %.2f%%, 差%+.2fpp) · 近100期 %.2f%%\n",
				s.StrName(), m.AllPct, m.BaseAll, m.AllPct-m.BaseAll, m.RecentAllPct)
		}
	}
}
