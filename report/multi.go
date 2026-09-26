// 多彩种页签生成：数字型（排列3/排列5/七星彩）+ 号码型（大乐透/七乐彩/快乐8）。
// 复用现有深色样式类，页签/面板 HTML 与对应 CSS 规则在本文件拼装，
// 由模板以 {{.MultiCSS}} / {{.MultiBar}} / {{.MultiPanes}} 注入。
package report

import (
	"fmt"
	"strings"

	"fc3d-kill6/backtest"
	"fc3d-kill6/data"
	"fc3d-kill6/engine/num"
)

// DigitView 数字型彩种页签视图
type DigitView struct {
	Game      data.GameDef
	Meta      backtest.Meta
	Pred      backtest.Predict
	NextIssue string
	Rows      []backtest.Row // 最新在前，取尾部 15 期
}

// NumView 号码型彩种页签视图
type NumView struct {
	Game      data.GameDef
	Meta      backtest.NumMeta  // 轻杀口径（高命中，主展示）
	Deep      *backtest.NumMeta // 深杀口径（强过滤，副展示），可为 nil
	NextIssue string
	Hot       []num.NumFreq // 近 20 期热号 top8
	Cold      []num.NumFreq // 近 20 期冷号 top8
	MissTop   []num.NumMiss // 遗漏 top8
}

// MultiViews 全部新增彩种视图
type MultiViews struct {
	Digits []*DigitView
	Nums   []*NumView
}

// 各彩种强调色（页签选中态）
var multiAccent = map[string]string{
	"pl3": "#22D3EE", "pl5": "#60A5FA", "qxc": "#FBBF24",
	"dlt": "#A78BFA", "qlc": "#34D399", "kl8": "#F87171",
}

var multiTabIco = map[string]string{
	"pl3": "排3", "pl5": "排5", "qxc": "七星", "dlt": "乐透", "qlc": "七乐", "kl8": "快8",
}

// multiCSS 生成各彩种页签选中态 CSS 规则
func (mv *MultiViews) multiCSS() string {
	var sb strings.Builder
	for _, dv := range mv.Digits {
		cssTab(&sb, dv.Game.Key)
	}
	for _, nv := range mv.Nums {
		cssTab(&sb, nv.Game.Key)
	}
	return sb.String()
}

func cssTab(sb *strings.Builder, key string) {
	c, ok := multiAccent[key]
	if !ok {
		c = "#22D3EE"
	}
	fmt.Fprintf(sb, `#tab-%[1]s:checked~.tab-bar .tab-btn[for="tab-%[1]s"]{color:%[2]s;border-color:%[2]s;background:rgba(18,26,43,.9)}
#tab-%[1]s:checked~.tab-bar .tab-btn[for="tab-%[1]s"] .tab-ico{color:%[2]s;border-color:%[2]s}
#tab-%[1]s:checked~#pane-%[1]s{display:block}
`, key, c)
}

// multiRadios 生成隐藏的 radio 选中开关（须位于各 pane 的兄弟位置）
func (mv *MultiViews) multiRadios() string {
	var sb strings.Builder
	for _, dv := range mv.Digits {
		fmt.Fprintf(&sb, `<input type="radio" name="lot" id="tab-%s">
`, dv.Game.Key)
	}
	for _, nv := range mv.Nums {
		fmt.Fprintf(&sb, `<input type="radio" name="lot" id="tab-%s">
`, nv.Game.Key)
	}
	return sb.String()
}

// multiBtns 生成页签按钮（注入 3D/双色球所在的同一 nav.tab-bar，保证单行排列）
func (mv *MultiViews) multiBtns() string {
	var sb strings.Builder
	for _, dv := range mv.Digits {
		fmt.Fprintf(&sb, `<label class="tab-btn" for="tab-%s"><span class="tab-ico">%s</span>%s</label>
`, dv.Game.Key, multiTabIco[dv.Game.Key], dv.Game.Name)
	}
	for _, nv := range mv.Nums {
		fmt.Fprintf(&sb, `<label class="tab-btn" for="tab-%s"><span class="tab-ico">%s</span>%s</label>
`, nv.Game.Key, multiTabIco[nv.Game.Key], nv.Game.Name)
	}
	return sb.String()
}

// multiPanes 生成全部页签面板
func (mv *MultiViews) multiPanes() string {
	var sb strings.Builder
	for _, dv := range mv.Digits {
		sb.WriteString(digitPane(dv))
	}
	for _, nv := range mv.Nums {
		sb.WriteString(numPane(nv))
	}
	return sb.String()
}

// 数字型面板：下期 6 杀码 + 命中率统计 + 近 15 期明细
func digitPane(dv *DigitView) string {
	m := dv.Meta
	var rowsSB strings.Builder
	for _, r := range dv.Rows {
		cls := "ok"
		if !r.All6OK {
			cls = "bad"
		}
		fmt.Fprintf(&rowsSB, `<tr><td class="issue-no">%s</td><td class="date">%s</td><td class="win-num">%s</td>`+
			`<td class="kill-code %s">%d, %d</td><td class="kill-code %s">%d, %d</td><td class="kill-code %s">%d, %d</td>`+
			`<td><span class="result %s">%s</span></td></tr>`,
			r.Issue, r.Date, r.Open,
			cls2(r.HOK, r.H2OK), r.HK, r.HK2,
			cls2(r.TOK, r.T2OK), r.TK, r.TK2,
			cls2(r.OOK, r.O2OK), r.OK, r.OK2,
			cls, boolZH(r.All6OK))
	}
	projNote := ""
	if dv.Game.NumLen > 3 {
		projNote = fmt.Sprintf("本彩种共 %d 位，杀码口径为末三位（百/十/个）。", dv.Game.NumLen)
	}
	return fmt.Sprintf(`<div class="tab-pane" id="pane-%s">
  <main>
    <section class="hero">
      <div class="hero-top">
        <div class="hero-left">
          <div class="kicker"><span class="tag">%s · 六杀制</span><span class="kicker-line">与福彩3D 同一套 V9.3 双引擎 · 末三位口径 · 3杀/6杀双档</span></div>
          <h1>%s 杀码参考</h1>
          <p class="hero-sub">与福彩3D 完全相同的双引擎（kill1 决策树 + kill2 算术公式）作用于本彩种每期末三位。3 杀口径（每位置排除 1 个）近 100 期全中率 %.1f%%，随机基线 72.9%%；6 杀口径（每位置排除 2 个，强过滤）全中率 %.1f%%，基线 51.2%%。杀得少命中高、杀得多过滤强，由组合数学决定。</p>
          <div class="plain-tip"><span class="pt-label">如实说明</span><span class="pt-body">同一套公式在不同彩种上的表现由数据决定，页面如实回测呈现，不做任何调整或美化。%s</span></div>
        </div>
        <div class="issue-badge">
          <span class="issue-label">下一期参考 · 第 %s 期</span>
          <span class="issue-value">6 杀码</span>
        </div>
      </div>
      <div class="pred-row">
        <div class="pred-card cyan"><div class="pred-head"><span class="pred-label">百位 · 双杀码</span><span class="mini-tag">kill1 + kill2</span></div><div class="pred-num">%d, %d</div><div class="pred-foot">近%d期命中 %.1f%%</div></div>
        <div class="pred-card violet"><div class="pred-head"><span class="pred-label">十位 · 双杀码</span><span class="mini-tag">kill1 + kill2</span></div><div class="pred-num">%d, %d</div><div class="pred-foot">近%d期命中 %.1f%%</div></div>
        <div class="pred-card amber"><div class="pred-head"><span class="pred-label">个位 · 双杀码</span><span class="mini-tag">kill1 + kill2</span></div><div class="pred-num">%d, %d</div><div class="pred-foot">近%d期命中 %.1f%%</div></div>
      </div>
    </section>
    <section class="section">
      <div class="section-head">
        <h2 class="section-title">近 %d 期回测</h2>
        <span class="section-meta">3杀全中 %.1f%% · 6杀全中 %.1f%% · 基线 72.9%%/51.2%% · 全量 %d 期</span>
      </div>
      <div class="bt-grid">
        <div class="cmp-cell"><span class="cmp-value v-green">%.1f%%</span><span class="cmp-label">3杀全中（基线 72.9%%）</span></div>
        <div class="cmp-cell"><span class="cmp-value v-text2">%.1f%%</span><span class="cmp-label">6杀全中（基线 51.2%%）</span></div>
        <div class="cmp-cell"><span class="cmp-value v-cyan">%.1f%%</span><span class="cmp-label">百位双杀</span></div>
        <div class="cmp-cell"><span class="cmp-value v-violet">%.1f%%</span><span class="cmp-label">十位双杀</span></div>
        <div class="cmp-cell"><span class="cmp-value v-amber">%.1f%%</span><span class="cmp-label">个位双杀</span></div>
      </div>
      <div class="wf-note">%s</div>
    </section>
    <section class="section">
      <div class="section-head">
        <h2 class="section-title">近 15 期回测明细</h2>
        <span class="section-meta">滚动窗口 · 每日自动更新</span>
      </div>
      <div class="table-wrap">
        <table>
          <thead><tr><th>期号</th><th>日期</th><th>开奖</th><th>百位杀码</th><th>十位杀码</th><th>个位杀码</th><th>6杀结果</th></tr></thead>
          <tbody>%s</tbody>
        </table>
      </div>
      <div class="warn" style="margin-top:20px"><span class="warn-icon">!</span><span>理性参考提示：彩票本质是随机游戏，杀码结果仅基于历史数据统计，不构成任何投注建议。请理性娱乐。</span></div>
    </section>
  </main>
</div>`,
		dv.Game.Key, dv.Game.Name, dv.Game.Name, m.AccPeriod100, m.Period6Pct100, projNote,
		dv.NextIssue,
		dv.Pred.H, dv.Pred.H2, m.BacktestN, m.AccH,
		dv.Pred.T, dv.Pred.T2, m.BacktestN, m.AccT,
		dv.Pred.O, dv.Pred.O2, m.BacktestN, m.AccO,
		m.BacktestN, m.AccPeriod100, m.Period6Pct100, m.Total,
		m.AccPeriod100, m.Period6Pct100, m.AccH, m.AccT, m.AccO,
		digitWFNote(m), rowsSB.String())
}

func digitWFNote(m backtest.Meta) string {
	if m.Total <= 100 {
		return fmt.Sprintf("全量共 %d 期，窗口尚小，指标波动较大。", m.Total)
	}
	return fmt.Sprintf("全量 %d 期长期收敛口径约 %.0f%%——滚动窗口会随开奖上下波动，与随机基线的偏离属正常波动范围。", m.Total, 51.2)
}

func cls2(a, b bool) string {
	if a && b {
		return "ok"
	}
	return "bad"
}

func boolZH(b bool) string {
	if b {
		return "全中"
	}
	return "未中"
}

// 号码型面板：双区杀号 + 冷热遗漏 + 近 15 期明细
func numPane(nv *NumView) string {
	m := nv.Meta
	areaDesc := fmt.Sprintf("主区 1-%d 选 %d · 杀 %d 个", numMaxA(nv), numPickA(nv), m.KillAN)
	if m.KillBN > 0 {
		areaDesc += fmt.Sprintf(" · 副区 1-%d 选 %d · 杀 %d 个", numMaxB(nv), numPickB(nv), m.KillBN)
	}
	killBHTML := ""
	if m.KillBN > 0 {
		nums := make([]string, 0, len(m.KillB))
		for _, n := range m.KillB {
			nums = append(nums, fmt.Sprintf(`<b class="nb bd">%02d</b>`, n))
		}
		killBHTML = fmt.Sprintf(`<div class="sk-group"><span class="sk-label bd">杀副区 · %d 个</span><span class="sk-nums">%s</span></div>`, m.KillBN, strings.Join(nums, ""))
	}
	killANums := make([]string, 0, len(m.KillA))
	for _, n := range m.KillA {
		killANums = append(killANums, fmt.Sprintf(`<b class="nb rd">%02d</b>`, n))
	}

	var hotSB, coldSB, missSB strings.Builder
	maxFreq, maxMiss := 1, 1
	for _, r := range nv.Hot {
		if r.Freq > maxFreq {
			maxFreq = r.Freq
		}
	}
	for _, r := range nv.MissTop {
		if r.Miss > maxMiss {
			maxMiss = r.Miss
		}
	}
	for _, r := range nv.Hot {
		fmt.Fprintf(&hotSB, `<div class="rank-row"><span class="rank-num">%02d</span><div class="rank-bar"><div class="rank-fill hot" style="width:%d%%"></div></div><span class="rank-freq">%d次</span></div>`, r.Num, pctW(r.Freq, maxFreq), r.Freq)
	}
	for _, r := range nv.Cold {
		fmt.Fprintf(&coldSB, `<div class="rank-row"><span class="rank-num">%02d</span><div class="rank-bar"><div class="rank-fill cold" style="width:%d%%"></div></div><span class="rank-freq">%d次</span></div>`, r.Num, pctW(r.Freq, maxFreq), r.Freq)
	}
	for _, r := range nv.MissTop {
		fmt.Fprintf(&missSB, `<div class="rank-row"><span class="rank-num">%02d</span><div class="rank-bar"><div class="rank-fill miss" style="width:%d%%"></div></div><span class="rank-freq">%d期</span></div>`, r.Num, pctW(r.Miss, maxMiss), r.Miss)
	}

	var rowsSB strings.Builder
	for _, r := range m.Rows {
		kills := make([]string, 0, len(r.KillA)+len(r.KillB))
		for _, n := range r.KillA {
			kills = append(kills, fmt.Sprintf("%02d", n))
		}
		killAStr := strings.Join(kills, ",")
		killBStr := "—"
		bcls := "ok"
		if m.KillBN > 0 {
			bkills := make([]string, 0, len(r.KillB))
			for _, n := range r.KillB {
				bkills = append(bkills, fmt.Sprintf("%02d", n))
			}
			killBStr = strings.Join(bkills, ",")
			if !r.BOK {
				bcls = "bad"
			}
		}
		fmt.Fprintf(&rowsSB, `<tr><td class="issue-no">%s</td><td class="date">%s</td><td class="win-num">%s</td>`+
			`<td class="kill-code %s">%s</td><td class="kill-code %s">%s</td>`+
			`<td><span class="result %s">%s</span></td></tr>`,
			r.Issue, r.Date, r.Open,
			clsIf(r.AOK), killAStr, bcls, killBStr,
			clsIf(r.AllOK), boolZH(r.AllOK))
	}

	bpctNote := "主区"
	if m.KillBN > 0 {
		bpctNote = "副区"
	}
	deepSection := ""
	if nv.Deep != nil {
		d := nv.Deep
		deepSection = fmt.Sprintf(`<section class="section">
      <div class="section-head">
        <h2 class="section-title">深杀模式 · 强过滤口径</h2>
        <span class="section-meta">杀 %d 主区 + %s · 随机基线 %.1f%%</span>
        <p class="section-note">人话：多杀几个号，过滤力更强但命中更低——命中率和基线同步下降是数学必然，不是算法变差。</p>
      </div>
      <div class="bt-grid">
        <div class="cmp-cell"><span class="cmp-value v-text2">%.1f%%</span><span class="cmp-label">深杀全避开（基线 %.1f%%）</span></div>
        <div class="cmp-cell"><span class="cmp-value v-text2">%.1f%%</span><span class="cmp-label">最近 100 期</span></div>
        <div class="cmp-cell"><span class="cmp-value v-text2">杀 %d 个</span><span class="cmp-label">对比轻杀 %d 个：命中 %.1f%%</span></div>
      </div>
    </section>`,
			d.KillAN, killBRegionLabel(*d), d.BaseAll,
			d.AllPct, d.BaseAll, d.RecentAllPct, d.KillAN, m.KillAN, m.AllPct)
	}
	return fmt.Sprintf(`<div class="tab-pane" id="pane-%s">
  <main>
    <section class="hero">
      <div class="hero-top">
        <div class="hero-left">
          <div class="kicker"><span class="tag">%s · 统计工具</span><span class="kicker-line">%s</span></div>
          <h1>%s 杀号参考</h1>
          <p class="hero-sub">%s 每期用近 %d 期冷热统计排除 %d 个主区号码%s，杀号命中率与随机基线对照展示——统计无法预判开奖，如实呈现。</p>
          <div class="plain-tip"><span class="pt-label">小白版</span><span class="pt-body">「杀号」= <strong>帮你排除掉</strong>的号码。红色区为杀主区，蓝色区为杀副区（如无副区则只有主区）。</span></div>
        </div>
        <div class="issue-badge">
          <span class="issue-label">最新开奖 · 第 %s 期</span>
          <span class="issue-value">%s</span>
        </div>
      </div>
      <div class="ssq-hero-card">
        <div class="shc-left">
          <span class="shc-label">杀号全避开率 · 全量 %d 期回测</span>
          <div class="shc-value">%.1f%%</div>
          <div class="shc-sub">杀 %d 主区 + %s</div>
          <div class="shc-meta">最近 100 期 <b>%.1f%%</b> · 随机基线 <b>%.1f%%</b></div>
        </div>
        <div class="shc-ring"></div>
      </div>
      <div class="ssq-kill">
        <div class="sk-head"><span class="sk-title">下期参考 · 第 %s 期</span><span class="sk-strategy">%s · 近%d期统计</span></div>
        <div class="sk-body">
          <div class="sk-group"><span class="sk-label rd">杀主区 · %d 个</span><span class="sk-nums">%s</span></div>
          %s
        </div>
        <div class="sk-note">全量回测：杀主区全避开 <b>%.1f%%</b>（基线 %.1f%%）%s全避开 <b>%.1f%%</b>（基线 %.1f%%）。与随机基线相当属预期结果，仅供参考。</div>
      </div>
    </section>
    %s
    <section class="section">
      <div class="section-head">
        <h2 class="section-title">冷热 · 遗漏</h2>
        <span class="section-meta">近 20 期统计</span>
      </div>
      <div class="ssq-grid">
        <div class="ssq-card"><div class="ssq-hd"><span>热号 Top8</span><span class="mini-tag">近20期</span></div>%s</div>
        <div class="ssq-card"><div class="ssq-hd"><span>冷号 Top8</span><span class="mini-tag">近20期</span></div>%s</div>
        <div class="ssq-card"><div class="ssq-hd"><span>遗漏 Top8</span><span class="mini-tag">最久没出</span></div>%s</div>
      </div>
    </section>
    <section class="section">
      <div class="section-head">
        <h2 class="section-title">近 15 期回测明细</h2>
        <span class="section-meta">%s · 策略 %s · 窗口 %d 期</span>
      </div>
      <div class="table-wrap">
        <table>
          <thead><tr><th>期号</th><th>日期</th><th>开奖</th><th>杀主区</th><th>杀副区</th><th>结果</th></tr></thead>
          <tbody>%s</tbody>
        </table>
      </div>
      <div class="warn" style="margin-top:20px"><span class="warn-icon">!</span><span>理性参考提示：彩票本质是随机游戏，杀号结果仅基于历史数据统计，不构成任何投注建议。请理性娱乐。</span></div>
    </section>
  </main>
</div>`,
		m.Game.Key, m.Game.Name, areaDesc, m.Game.Name, m.Game.Name,
		m.Window, m.KillAN, killBRegionLabel(m),
		m.LatestIssue, m.LatestDate,
		m.Total, m.AllPct, m.KillAN, killBRegionLabel(m), m.RecentAllPct, m.BaseAll,
		m.NextIssue, m.Strategy, m.Window,
		m.KillAN, strings.Join(killANums, ""), killBHTML,
		m.APct, m.BaseA, bpctNote, m.AllPct, m.BaseAll,
		hotSB.String(), coldSB.String(), missSB.String(),
		deepSection,
		areaDesc, m.Strategy, m.Window, rowsSB.String())
}

func killBRegionLabel(m backtest.NumMeta) string {
	if m.KillBN > 0 {
		return fmt.Sprintf("杀 %d 副区", m.KillBN)
	}
	return "不杀副区"
}

func numMaxA(nv *NumView) int  { return areaMax(nv.Game, 0) }
func numPickA(nv *NumView) int { return areaPick(nv.Game, 0) }
func numMaxB(nv *NumView) int  { return areaMax(nv.Game, 1) }
func numPickB(nv *NumView) int { return areaPick(nv.Game, 1) }
func clsIf(ok bool) string {
	if ok {
		return "ok"
	}
	return "bad"
}

// areaMax / areaPick 各彩种分区参数（与 backtest.NumCfgFor 一致）
func areaMax(g data.GameDef, area int) int {
	switch g.Key {
	case "dlt":
		if area == 0 {
			return 35
		}
		return 12
	case "qlc":
		return 30
	case "kl8":
		return 80
	}
	return g.MaxVal
}

func areaPick(g data.GameDef, area int) int {
	switch g.Key {
	case "dlt":
		if area == 0 {
			return 5
		}
		return 2
	case "qlc":
		if area == 0 {
			return 7
		}
		return 1
	case "kl8":
		return 20
	}
	return g.NumLen
}

// pctW 频率条宽百分比（0-100）
func pctW(v, max int) int {
	if max <= 0 {
		return 0
	}
	p := v * 100 / max
	if p > 100 {
		p = 100
	}
	return p
}
