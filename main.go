// fc3d-kill6 — 福彩3D 百十个杀码预测 CLI（V9.3 六杀制，Go 实现）
//
// 数据流：抓数(灰鸟/17500 双源) → 期号校验 → 追加CSV → V9引擎回测
//
//	→ kill6监控+升级检测 → 生成 index.html
//
// 部署形态兼容：GitHub Pages(静态产物) / Docker(单二进制) / Serverless(引擎纯函数)。
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"fc3d-kill6/backtest"
	"fc3d-kill6/data"
	"fc3d-kill6/engine/pattern"
	"fc3d-kill6/engine/ssq"
	"fc3d-kill6/fetch"
	"fc3d-kill6/monitor"
	"fc3d-kill6/report"
)

func main() {
	csvPath := flag.String("csv", "fc3d-history.csv", "福彩3D CSV 路径")
	ssqCSVPath := flag.String("ssq-csv", "ssq-history.csv", "双色球 CSV 路径")
	htmlPath := flag.String("html", "index.html", "输出 HTML 路径")
	kill6Path := flag.String("kill6", "kill6_history.json", "kill6 监控历史 JSON 路径")
	flag.Parse()

	fmt.Println("=" + repeat("=", 30))
	fmt.Println("福彩3D 六杀 + 双色球统计 · 云端更新 (Go)")
	fmt.Println("=" + repeat("=", 30))

	// Step 1: 获取最新开奖
	fmt.Println("📡 获取最新开奖...")
	newData, dataAlive := fetch.FetchLatest(*csvPath)
	if newData != nil {
		added, err := data.AppendCSV(*csvPath, data.Draw{
			Issue: newData.Issue, Date: newData.Date,
			B: newData.B, S: newData.S, G: newData.G,
		})
		if err != nil {
			fmt.Printf("  ❌ 追加失败: %v\n", err)
		} else if added == 1 {
			fmt.Printf("  ✅ 已追加第%s期 (%s) %d%d%d\n", newData.Issue, newData.Date, newData.B, newData.S, newData.G)
		} else {
			fmt.Printf("  ℹ️ 第%s期已存在, 无需追加\n", newData.Issue)
		}
	} else if !dataAlive {
		fmt.Println("\n🚨🚨🚨 所有数据源均失败! 页面将显示旧数据, 请检查数据源 🚨🚨🚨")
	} else {
		fmt.Println("  ℹ️ 数据源正常但无新一期(开奖前运行), 继续用现有数据")
	}

	// Step 2: 加载数据
	draws, err := data.LoadCSV(*csvPath)
	if err != nil || len(draws) < 100 {
		fmt.Printf("❌ 数据不足或读取失败: %v (%d期)\n", err, len(draws))
		os.Exit(1)
	}

	// Step 3: 回测
	bt := backtest.RunAll(draws)
	m := bt.Meta
	fmt.Printf("\n📊 回测 %d期: 百%.1f%% 十%.1f%% 个%.1f%% 综合%.1f%%\n",
		m.BacktestN, m.AccH, m.AccT, m.AccO, m.AccAll)
	fmt.Printf("   kill2: 百%.1f%% 十%.1f%% 个%.1f%%\n", m.AccH2, m.AccT2, m.AccO2)
	fmt.Printf("   6杀全中: %d/%d = %.1f%%\n", m.All6, m.BacktestN, m.All6Pct)
	fmt.Printf("   近100期综合(按期): %d/%d = %.1f%%\n", m.PeriodCorrect100, m.PeriodN100, m.AccPeriod100)
	fmt.Printf("   近100期6杀全中: %d/%d = %.1f%%\n", m.Period6Correct100, m.PeriodN100, m.Period6Pct100)

	// Step 3.5: 多窗口回测（本地诊断：窗口独立重置状态机，含个位自适应）
	fmt.Printf("\n📈 多窗口回测 (3杀综合/6杀全中/最大连错):\n")
	for w, ws := range backtest.MultiWindow(draws, []int{100, 200, 300, 500}) {
		fmt.Printf("   %s: 综合%.1f%% | 6杀全中%.1f%% | 最大连错%d期\n",
			w, ws.Overall, ws.All6Pct, ws.MaxConsecutive)
	}

	// Step 3.6: walk-forward 滚动前推验证（无泄漏，对照随机基线 51.2% 二项检验）
	fmt.Printf("\n📉 Walk-forward 滚动前推验证 (窗口独立重置状态机, 基线 %.1f%%):\n", backtest.BasePct*100)
	wf := backtest.WalkForward(draws, []int{100, 200, 300, 500})
	for _, w := range wf {
		pv := fmt.Sprintf("%.4f", w.PVal)
		if w.PVal < 0.001 {
			pv = "<0.001"
		}
		fmt.Printf("   %s: 6杀全中%d/%d=%.1f%% | 超越%+.1fpp | z=%.1f p=%s\n",
			w.Label, w.All6, w.N, w.All6Pct, w.BeatPP, w.Z, pv)
	}

	// Step 3.7: 规律挖掘（胆码/毒胆/和尾，跨期组合，算法移植自 lottery-analyzer）
	fmt.Println("\n🔎 规律挖掘 (跨期组合参考)...")
	pat := pattern.Backtest(drawsToPattern(draws), pattern.Default, 100)
	for _, s := range pat.Stats {
		picks := joinInts(pat.Latest[s.Kind].Picks)
		if picks == "" {
			picks = "暂无"
		}
		fmt.Printf("   %s: 近%d期命中%.1f%% (基线%.1f%%) | 全量%.1f%% | 本期%s\n",
			s.Kind.Short(), s.N, s.Rate, s.Base, s.FullRate, picks)
	}

	// Step 4: 趋势记录与表现预警
	hist, _ := monitor.Record(*kill6Path, m.Period6Pct100, m.LatestIssue, m.LatestDate)
	triggered, reasons, monthDrop := monitor.CheckAlert(m.Period6Pct100, hist)
	if triggered {
		fmt.Println("\n🚨🚨🚨 算法表现预警：滚动 100 期 6 杀全中率明显下滑 🚨🚨🚨")
		for _, r := range reasons {
			fmt.Printf("   ⚠️ %s\n", r)
		}
	} else {
		fmt.Printf("   ✅ 表现监控: 正常 (单月%+.1fpp, 预警阈值跌破70%%/月降8pp)\n", monthDrop)
	}

	// Step 4.5: 双色球（统计工具版）
	fmt.Println("\n🎱 双色球统计...")
	ssqData, ssqAlive := fetch.FetchLatestSSQ(*ssqCSVPath)
	if ssqData != nil {
		added, err := data.AppendSSQCSV(*ssqCSVPath, data.SSQDraw{
			Issue: ssqData.Issue, Date: ssqData.Date,
			R1: ssqData.Reds[0], R2: ssqData.Reds[1], R3: ssqData.Reds[2],
			R4: ssqData.Reds[3], R5: ssqData.Reds[4], R6: ssqData.Reds[5],
			Blue: ssqData.Blue,
		})
		if err != nil {
			fmt.Printf("  ❌ 双色球追加失败: %v\n", err)
		} else if added == 1 {
			fmt.Printf("  ✅ 双色球已追加第%s期 (%s)\n", ssqData.Issue, ssqData.Date)
		} else {
			fmt.Printf("  ℹ️ 双色球第%s期已存在\n", ssqData.Issue)
		}
	} else if !ssqAlive {
		fmt.Println("  ⚠️ 双色球数据源全挂，继续用旧数据")
	}
	ssqDraws, err := data.LoadSSQCSV(*ssqCSVPath)
	if err != nil || len(ssqDraws) < 300 {
		fmt.Printf("  ❌ 双色球数据不足: %v (%d 期)\n", err, len(ssqDraws))
		os.Exit(1)
	}
	ssqStrategy := ssq.StrategyHot // 全量回测中相对最稳（见 ssq_probe）
	ssqWin := 50
	ssqRes := backtest.SSQBacktest(ssqDraws, 6, 3, ssqWin, ssqStrategy)
	ssqRes.Meta.NextIssue = ssqNextIssue(ssqData, ssqRes.Meta.LatestIssue, ssqRes.Meta.LatestDate)
	ssqView := buildSSQView(ssqRes, ssqDraws)
	fmt.Printf("  📊 杀%d红+杀%d蓝: 全中%.1f%% (基线%.1f%%) · 最新期 %s\n",
		ssqRes.Meta.RedN, ssqRes.Meta.BlueN, ssqRes.Meta.AllPct, ssqRes.Meta.BaseAll, ssqRes.Meta.LatestIssue)

	// Step 5: 生成 HTML
	nextIssue := fetch.NextIssueCalc(m.LatestIssue, m.LatestDate, nextIssueHint(newData))
	banners := report.Banners{DataFailed: !dataAlive}
	html, err := report.GenerateHTML(m, bt.Pred, bt.Rows, banners, nextIssue, wf, ssqView, pat)
	if err != nil {
		fmt.Printf("❌ HTML 生成失败: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*htmlPath, []byte(html), 0o644); err != nil {
		fmt.Printf("❌ 写入 HTML 失败: %v\n", err)
		os.Exit(1)
	}

	p := bt.Pred
	fmt.Printf("\n🔮 3D下一期: %s | 百杀%d,%d 十杀%d,%d 个杀%d,%d\n",
		nextIssue, p.H, p.H2, p.T, p.T2, p.O, p.O2)
	fmt.Printf("✅ HTML已生成 (%s, %d字节)\n", *htmlPath, len(html))
}

// buildSSQView 组装双色球页签视图数据（统计榜 + 和值走势）
func buildSSQView(res backtest.SSQResult, draws []data.SSQDraw) *report.SSQView {
	const statWin = 20
	hot := ssq.FreqReds(draws, statWin)
	cold := make([]ssq.NumFreq, 0, len(hot))
	// 冷号 = 频率升序（出现少的）
	for i := len(hot) - 1; i >= 0; i-- {
		cold = append(cold, hot[i])
	}
	miss := ssq.MissReds(draws, statWin)
	blue := ssq.FreqBlues(draws, statWin)
	sum := ssq.SumSeries(draws, statWin)
	sumAvg := 0
	if len(sum) > 0 {
		t := 0
		for _, v := range sum {
			t += v
		}
		sumAvg = t / len(sum)
	}
	maxFreq := 1
	for _, r := range hot {
		if r.Freq > maxFreq {
			maxFreq = r.Freq
		}
	}
	missMax := 1
	for _, r := range miss {
		if r.Miss > missMax {
			missMax = r.Miss
		}
	}
	// 保留池：33 个红球去掉本期杀的 6 个
	keep := []int{}
	for n := 1; n <= 33; n++ {
		excluded := false
		for _, k := range res.Meta.KillReds {
			if k == n {
				excluded = true
				break
			}
		}
		if !excluded {
			keep = append(keep, n)
		}
	}
	return &report.SSQView{
		Meta:    res.Meta,
		HotReds: hot, ColdReds: cold, MissReds: miss, BlueFreq: blue,
		SumTrend: sum, SumAvg: sumAvg,
		MaxFreq: maxFreq, MissMax: missMax,
		KeepReds: keep,
	}
}

// nextIssueHint 透传数据源 next_code（跨年安全）
func nextIssueHint(lt *fetch.Latest) string {
	if lt != nil {
		return lt.NextIssue
	}
	return ""
}

// ssqNextIssue 双色球下一期期号：数据源 next_code 优先，兜底按最新期号自增（跨年安全）
func ssqNextIssue(lt *fetch.LatestSSQ, issue, date string) string {
	if lt != nil && lt.NextIssue != "" {
		return lt.NextIssue
	}
	return fetch.NextIssueCalc(issue, date, "")
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}

// drawsToPattern 转换 data.Draw → pattern.Draw
func drawsToPattern(ds []data.Draw) []pattern.Draw {
	out := make([]pattern.Draw, len(ds))
	for i, d := range ds {
		out[i] = pattern.Draw{Issue: d.Issue, B: d.B, S: d.S, G: d.G}
	}
	return out
}

func joinInts(a []int) string {
	parts := make([]string, len(a))
	for i, v := range a {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return strings.Join(parts, ",")
}
