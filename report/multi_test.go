package report

import (
	"strings"
	"testing"

	"fc3d-kill6/backtest"
	"fc3d-kill6/data"
	"fc3d-kill6/engine/num"
)

func TestGenerateHTMLWithMulti(t *testing.T) {
	html, err := GenerateHTML(multiTestMeta(), backtest.Predict{}, nil, Banners{}, "2026223", nil, minSSQView(), nil, multiTestViews())
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range []string{"pl3", "dlt"} { // 测试视图仅含这两个彩种
		if !strings.Contains(html, `id="tab-`+g+`"`) {
			t.Errorf("缺少页签 %s", g)
		}
		if !strings.Contains(html, `id="pane-`+g+`"`) {
			t.Errorf("缺少面板 %s", g)
		}
		if !strings.Contains(html, `#tab-`+g+`:checked~#pane-`+g+`{display:block}`) {
			t.Errorf("缺少 %s 选中态 CSS", g)
		}
	}
	if strings.Contains(html, "{{") {
		t.Error("模板占位符泄漏")
	}
}

func multiTestMeta() backtest.Meta {
	m := backtest.Meta{Total: 500, BacktestN: 100}
	m.AccH, m.AccT, m.AccO, m.AccAll = 60, 60, 60, 60
	m.Period6Pct100 = 64
	m.PeriodN100 = 100
	return m
}

func multiTestViews() *MultiViews {
	g3, _ := data.GameByKey("pl3")
	gn, _ := data.GameByKey("dlt")
	mv := &MultiViews{
		Digits: []*DigitView{{
			Game: g3, NextIssue: "2026259",
			Meta: multiTestMeta(), Pred: backtest.Predict{H: 1, H2: 2, T: 3, T2: 4, O: 5, O2: 6},
			Rows: []backtest.Row{{Issue: "2026258", Date: "2026-09-25", Open: "137", HK: 5, HK2: 2, TK: 1, TK2: 3, OK: 5, OK2: 6, All6OK: true}},
		}},
		Nums: []*NumView{{
			Game: gn, NextIssue: "26110",
			Meta: backtest.NumMeta{
				Game: gn, Total: 2927, LatestIssue: "26109", LatestDate: "2026-09-23",
				Strategy: "热号法", Window: 50, KillA: []int{1, 2, 3, 4, 5}, KillB: []int{6},
				KillAN: 5, KillBN: 1,
				APct: 40, BPct: 80, AllPct: 35, BaseA: 43.9, BaseB: 83.3, BaseAll: 36.6,
				RecentAPct: 40, RecentBPct: 80, RecentAllPct: 35,
			},
			Hot:     []num.NumFreq{{Num: 1, Freq: 3}, {Num: 2, Freq: 2}},
			Cold:    []num.NumFreq{{Num: 30, Freq: 0}},
			MissTop: []num.NumMiss{{Num: 33, Miss: 12}},
		}},
	}
	return mv
}
