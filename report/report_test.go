package report

import (
	"strings"
	"testing"

	"fc3d-kill6/backtest"
	"fc3d-kill6/engine/pattern"
	"fc3d-kill6/engine/ssq"
)

func TestGenerateHTMLCore(t *testing.T) {
	m := backtest.Meta{Total: 8730, LatestIssue: "2026222", LatestDate: "2026-08-20", BacktestN: 100, Period6Pct100: 81.0}
	pred := backtest.Predict{H: 2, T: 3, O: 0, H2: 4, T2: 0, O2: 3}
	rows := []backtest.Row{
		{Issue: "2026222", Date: "2026-08-20", Open: "380", HK: 3, TK: 4, OK: 2, All6OK: true},
		{Issue: "2026221", Date: "2026-08-19", Open: "296", HK: 4, TK: 1, OK: 6, All6OK: false},
		{Issue: "2026220", Date: "2026-08-18", Open: "373", HK: 5, TK: 3, OK: 5, All6OK: true},
	}
	wf := []backtest.WFWindow{{Label: "100期", N: 100, All6: 81, All6Pct: 81.0, BeatPP: 29.8, Z: 6.0, PVal: 0.0001}}

	html, err := GenerateHTML(m, pred, rows, Banners{}, "2026223", wf, minSSQView(), nil, nil)
	if err != nil {
		t.Fatalf("GenerateHTML: %v", err)
	}
	for _, want := range []string{
		"福彩3D 百十个杀码参考",
		"2026222", "380",
		"wu529778790/fc3d-kill6", // GitHub 图标
		"wx-auth-sdk",            // 认证接入
		"Walk-forward 滚动验证",      // walk-forward 摘要
		"polyline",               // 趋势图折线
		"小白版",                    // 白话解释卡
		"排除掉的号码",                 // 术语表：杀=排除
		"彩票小白",                   // 术语表折叠入口
		"人话：",                    // 各 section 就近人话注释
		">福彩3D</label>",          // tab 标签
		">双色球</label>",           // tab 标签
	} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML 缺少关键内容: %q", want)
		}
	}
}

func TestGenerateHTMLWithSSQ(t *testing.T) {
	m := backtest.Meta{Total: 8730, LatestIssue: "2026222", LatestDate: "2026-08-20", BacktestN: 100, Period6Pct100: 81.0}
	pred := backtest.Predict{H: 2, T: 3, O: 0, H2: 4, T2: 0, O2: 3}
	rows := []backtest.Row{{Issue: "2026222", Date: "2026-08-20", Open: "380", All6OK: true}}
	sv := minSSQView()
	html, err := GenerateHTML(m, pred, rows, Banners{}, "2026223", nil, sv, nil, nil)
	if err != nil {
		t.Fatalf("GenerateHTML: %v", err)
	}
	for _, want := range []string{
		"双色球 杀号参考",
		"杀红球 · 6 个", "杀蓝球 · 3 个",
		"红球热号 Top6", "红球冷号 Top6", "红球遗漏 Top6", "蓝球频率",
		"红球和值走势",
		"策略 vs 随机基线",
		"双色球术语表",
		"rank-bar", // 榜条渲染
	} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML 缺少双色球内容: %q", want)
		}
	}
}

// minSSQView 最小可用的双色球视图（测试用）
func minSSQView() *SSQView {
	return &SSQView{
		Meta: backtest.SSQMeta{
			Total: 3493, LatestIssue: "2026096", LatestDate: "2026-08-20",
			Strategy: "热号法", Window: 50,
			KillReds: []int{1, 2, 3, 4, 5, 6}, KillBlues: []int{1, 2, 3},
			RedPct: 26.7, BluePct: 81.2, AllPct: 21.7,
			BaseRed: 26.7, BaseBlue: 81.2, BaseAll: 21.7,
			WF: []backtest.WFWindow{{Label: "100期", N: 100, All6Pct: 21.0, PVal: 0.5}},
		},
		HotReds: []ssq.NumFreq{{Num: 1, Freq: 5}}, ColdReds: []ssq.NumFreq{{Num: 33, Freq: 1}},
		MissReds: []ssq.NumMiss{{Num: 33, Miss: 20}}, BlueFreq: []ssq.NumFreq{{Num: 1, Freq: 3}},
		SumTrend: []int{100, 102, 98}, SumAvg: 102, MaxFreq: 5, MissMax: 20,
	}
}

func TestSSQSumSVG(t *testing.T) {
	svg := ssqSumSVG([]int{100, 102, 98, 105}, 102)
	if !strings.Contains(svg, "均值 102") || !strings.Contains(svg, "polyline") {
		t.Errorf("和值图缺少均值/折线: %s", svg)
	}
	if ssqSumSVG(nil, 0) != "" {
		t.Errorf("空数据应返回空串")
	}
}

func TestTrendSVG(t *testing.T) {
	rows := []backtest.Row{
		{All6OK: true}, {All6OK: false}, {All6OK: true}, {All6OK: true}, {All6OK: true},
	}
	svg := trendSVG(rows)
	if !strings.Contains(svg, "51.2") {
		t.Errorf("趋势图缺少随机基线: %s", svg)
	}
	if strings.Contains(svg, "70%") || strings.Contains(svg, "#F87171") {
		t.Errorf("趋势图不应再包含 70%% 红色预警线: %s", svg)
	}
	if !strings.Contains(svg, "polyline") {
		t.Errorf("趋势图缺少 polyline: %s", svg)
	}
	if trendSVG(nil) != "" {
		t.Errorf("空数据应返回空串")
	}
}

func TestWFNote(t *testing.T) {
	if wfNote(nil) != "" {
		t.Errorf("空 walk-forward 应返回空串")
	}
	note := wfNote([]backtest.WFWindow{{Label: "100期", N: 100, All6: 81, All6Pct: 81.0, BeatPP: 29.8, Z: 6.0, PVal: 0.00001}})
	if !strings.Contains(note, "p=<0.001") {
		t.Errorf("p<0.001 应显示为 <0.001, got: %s", note)
	}
}

// TestGenerateHTMLWithPattern 规律挖掘区块渲染
func TestGenerateHTMLWithPattern(t *testing.T) {
	m := backtest.Meta{Total: 8730, LatestIssue: "2026222", LatestDate: "2026-08-20", BacktestN: 100, Period6Pct100: 81.0}
	pred := backtest.Predict{H: 2, T: 3, O: 0, H2: 4, T2: 0, O2: 3}
	rows := []backtest.Row{{Issue: "2026222", Date: "2026-08-20", Open: "380", All6OK: true}}
	pr := &pattern.BacktestResult{
		Window: 100,
		Stats: []pattern.KindStats{
			{Kind: pattern.Danma, N: 100, Hit: 65, Rate: 65.0, Base: 65.7},
			{Kind: pattern.Dudan, N: 100, Hit: 34, Rate: 34.0, Base: 34.3},
			{Kind: pattern.SumBH, N: 100, Hit: 90, Rate: 90.0, Base: 90.0},
			{Kind: pattern.SumBT, N: 100, Hit: 91, Rate: 91.0, Base: 90.0},
			{Kind: pattern.SumTO, N: 100, Hit: 89, Rate: 89.0, Base: 90.0},
		},
		Latest: map[pattern.Kind]*pattern.Analysis{
			pattern.Danma: {Kind: pattern.Danma, HitCount: 2, Picks: []int{3, 7, 9},
				Hits: []pattern.Hit{{Path: "1_1|2_2", MaxCons: 5, Next: []int{3, 7}}}},
			pattern.Dudan: {Kind: pattern.Dudan, HitCount: 1, Picks: []int{0, 1, 2}},
			pattern.SumBH: {Kind: pattern.SumBH, HitCount: 1, Picks: []int{5}},
			pattern.SumBT: {Kind: pattern.SumBT, HitCount: 1, Picks: []int{6}},
			pattern.SumTO: {Kind: pattern.SumTO, HitCount: 1, Picks: []int{7}},
		},
	}
	html, err := GenerateHTML(m, pred, rows, Banners{}, "2026223", nil, minSSQView(), pr, nil)
	if err != nil {
		t.Fatalf("GenerateHTML: %v", err)
	}
	for _, want := range []string{
		"规律挖掘 · 跨期组合参考",
		"胆码参考", "毒胆参考", "和尾杀号",
		"至少一位开出", "全部不开",
		"百个和尾", "百十和尾", "十个和尾",
		"pat-card", "pat-num", "pat-detail",
		"规律明细 · 2 条", // Danma 明细计数
		"1_1|2_2",    // 规律路径展示
	} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML 缺少规律挖掘内容: %q", want)
		}
	}
}
