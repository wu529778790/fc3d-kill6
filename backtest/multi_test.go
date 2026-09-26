package backtest

import (
	"testing"

	"fc3d-kill6/data"
	"fc3d-kill6/engine/num"
)

// synthDigits 生成 200 期伪随机数字型开奖（LCG，可复现）
func synthDigits(n, digits int) []data.NumDraw {
	seed := 42
	out := make([]data.NumDraw, 0, n)
	for i := 0; i < n; i++ {
		seed = (seed*1103515245 + 12345) & 0x7fffffff
		nums := make([]int, digits)
		for j := range nums {
			seed = (seed*1103515245 + 12345) & 0x7fffffff
			nums[j] = seed % 10
		}
		out = append(out, data.NumDraw{
			Issue: itoa(2026000 + i), Date: "2026-01-01",
			Nums: nums,
		})
	}
	return out
}

func TestDigitBacktest(t *testing.T) {
	draws := synthDigits(200, 5) // 排列5 形态
	res, win, wf := DigitBacktest(draws)
	if res.Meta.Total != 200 {
		t.Fatalf("Total = %d", res.Meta.Total)
	}
	if len(res.Rows) != 100 {
		t.Fatalf("Rows = %d", len(res.Rows))
	}
	if res.Meta.BacktestN != 100 {
		t.Fatalf("BacktestN = %d", res.Meta.BacktestN)
	}
	if len(win) == 0 || len(wf) == 0 {
		t.Fatalf("多窗口/walk-forward 为空: %d %d", len(win), len(wf))
	}
	// 杀码必须在 0-9
	p := res.Pred
	for _, k := range []int{p.H, p.T, p.O, p.H2, p.T2, p.O2} {
		if k < 0 || k > 9 {
			t.Fatalf("杀码超界: %d", k)
		}
	}
}

func TestNumBacktestConsistency(t *testing.T) {
	g := data.GameDef{Key: "dlt", Name: "大乐透", NumLen: 7, MaxVal: 35, MinVal: 1}
	seed := 7
	draws := make([]data.NumDraw, 0, 120)
	for i := 0; i < 120; i++ {
		nums := []int{}
		for len(nums) < 5 {
			seed = (seed*1103515245 + 12345) & 0x7fffffff
			v := seed%35 + 1
			dup := false
			for _, x := range nums {
				if x == v {
					dup = true
					break
				}
			}
			if !dup {
				nums = append(nums, v)
			}
		}
		seed = (seed*1103515245 + 12345) & 0x7fffffff
		b1 := seed%12 + 1
		seed = (seed*1103515245 + 12345) & 0x7fffffff
		b2 := seed%12 + 1
		if b2 == b1 {
			b2 = b2%12 + 1
		}
		nums = append(nums, b1, b2)
		draws = append(draws, data.NumDraw{Issue: itoa(26000 + i), Date: "2026-01-01", Nums: nums})
	}
	cfg, _ := NumCfgFor(g)
	res := NumBacktest(draws, g, cfg)
	m := res.Meta
	if m.Total != 120 || len(m.Rows) != 70 { // 120 期 - 50 期窗口 = 70 行回测明细
		t.Fatalf("Total=%d Rows=%d", m.Total, len(m.Rows))
	}
	if m.BaseA <= 0 || m.BaseA >= 100 || m.BaseAll <= 0 || m.BaseAll >= 100 {
		t.Fatalf("基线异常: A=%.2f All=%.2f", m.BaseA, m.BaseAll)
	}
	if len(m.KillA) != 1 { // 轻杀口径：主区杀 1 个
		t.Fatalf("轻杀配置异常: %v", m.KillA)
	}
	deepCfg, _ := NumCfgDeepFor(g)
	dm := NumBacktest(draws, g, deepCfg).Meta
	if len(dm.KillA) != 5 { // 深杀口径：主区杀 5 个
		t.Fatalf("深杀配置异常: %v", dm.KillA)
	}
	if m.BaseAll <= dm.BaseAll { // 轻杀基线必须高于深杀
		t.Fatalf("轻杀基线 %.2f 应高于深杀基线 %.2f", m.BaseAll, dm.BaseAll)
	}
	for _, r := range m.Rows {
		if r.AllOK != (r.AOK && r.BOK) {
			t.Fatal("AllOK 应等于 AOK && BOK")
		}
	}
}

func TestNumCfgFor(t *testing.T) {
	for _, key := range []string{"dlt", "qlc", "kl8"} {
		g, _ := data.GameByKey(key)
		if _, ok := NumCfgFor(g); !ok {
			t.Fatalf("%s 缺少回测配置", key)
		}
	}
	if _, ok := NumCfgFor(data.GameDef{Key: "pl3"}); ok {
		t.Fatal("数字型不应有号码型配置")
	}
	_ = num.StrategyHot.StrName()
}
