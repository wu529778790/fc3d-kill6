// Package report 生成数据参考页面 HTML（深色数据大屏响应式，内联样式零外部依赖）。
package report

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"text/template"

	"fc3d-kill6/backtest"
	"fc3d-kill6/engine/pattern"
	"fc3d-kill6/engine/ssq"
)

// Data 模板数据
type Data struct {
	Meta    backtest.Meta
	Pred    backtest.Predict
	Rows    []backtest.Row
	Banners Banners
}

// Banners 页面顶部横幅
type Banners struct {
	DataFailed bool // 数据源全挂（橙条）
}

// SSQView 双色球页签数据（统计工具版）
type SSQView struct {
	Meta     backtest.SSQMeta
	HotReds  []ssq.NumFreq // 近 20 期热号
	ColdReds []ssq.NumFreq // 近 20 期冷号
	MissReds []ssq.NumMiss // 遗漏榜
	BlueFreq []ssq.NumFreq // 蓝球频率
	SumTrend []int         // 近 20 期红球和值
	SumAvg   int           // 和值均值
	MaxFreq  int           // 红球最大频率（榜条宽度基准）
	MissMax  int           // 遗漏榜最大遗漏（榜条宽度基准）
	KeepReds []int         // 本期红球保留池（33 减去杀 6）
}

// PatView 规律挖掘区块视图数据（跨期组合参考）
type PatView struct {
	Window int               // 回测窗口（近 N 期）
	Danma  *pattern.Analysis // 胆码
	Dudan  *pattern.Analysis // 毒胆
	SumBH  *pattern.Analysis // 杀百个和尾
	SumBT  *pattern.Analysis // 杀百十和尾
	SumTO  *pattern.Analysis // 杀十个和尾

	DanmaSt, DudanSt          *pattern.KindStats
	SumBHSt, SumBTSt, SumTOSt *pattern.KindStats
}

// view 模板视图（Rows 已按最新在前排序）
type view struct {
	Meta      backtest.Meta
	Pred      backtest.Predict
	Rows      []backtest.Row
	MCards    []backtest.Row
	Banners   Banners
	NextIssue string
	Ring      string
	Pct6Beat  float64
	WFNote    string
	TrendSVG  string
	SSQRing   string
	SSQ       *SSQView
	Pat       *PatView

	MultiCSS   string // 多彩种页签 CSS（含选中态规则）
	MultiBar   string // 多彩种页签 radio（隐藏式选中开关）
	MultiBtns  string // 多彩种页签按钮（与 3D/双色球同一个 nav 内）
	MultiPanes string // 多彩种页签面板
}

// ringSVG 生成 6 杀全中率环形进度（path 圆弧，规避 transform 解析问题）
func ringSVG(pct float64) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	const size, r, sw = 96.0, 38.0, 9.0
	cx, cy := size/2, size/2
	theta := pct * 3.6 * math.Pi / 180
	ex := cx + r*math.Cos(theta)
	ey := cy + r*math.Sin(theta)
	large := 0
	if pct > 50 {
		large = 1
	}
	inner := int(math.Round(pct))
	return fmt.Sprintf(`<svg class="ring" width="96" height="96" viewBox="0 0 96 96" fill="none" xmlns="http://www.w3.org/2000/svg"><circle cx="48" cy="48" r="38" stroke="#1E293B" stroke-width="9"/><path d="M86 48A38 38 0 %d 1 %.1f %.1f" stroke="#34D399" stroke-width="9" stroke-linecap="round"/><text x="48" y="55" text-anchor="middle" font-size="21" font-weight="700" fill="#E8EEF9" font-family="SF Mono,monospace">%d%%</text></svg>`, large, ex, ey, inner)
}

// reverseRows 返回 rows 的反转副本（最新在前）
func reverseRows(rows []backtest.Row) []backtest.Row {
	out := make([]backtest.Row, len(rows))
	for i, r := range rows {
		out[len(rows)-1-i] = r
	}
	return out
}

const tmplSrc = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1.0">
<meta name="color-scheme" content="dark">
<title>福彩3D 杀码 + 双色球 数据参考</title>
<style>
:root{--bg1:#0A0E17;--bg2:#0D1424;--surface:#121A2B;--surface2:#0D1424;--border:#1E293B;--border-soft:rgba(148,163,184,.12);--text1:#E8EEF9;--text2:#94A3B8;--text3:#64748B;--cyan:#22D3EE;--cyan-soft:#67E8F9;--violet:#A78BFA;--violet-soft:#C4B5FD;--amber:#FBBF24;--amber-soft:#FCD34D;--blue:#60A5FA;--blue-soft:#93C5FD;--green:#34D399;--green-soft:#6EE7B7;--red:#F87171;--radius:16px;--font-cn:"PingFang SC","Hiragino Sans GB","Microsoft YaHei",-apple-system,sans-serif;--font-num:"SF Mono",ui-monospace,"JetBrains Mono",Menlo,monospace}
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:var(--font-cn);background:linear-gradient(180deg,var(--bg1),var(--bg2));color:var(--text1);min-height:100vh;-webkit-font-smoothing:antialiased}
.bg-grid{position:fixed;inset:0;z-index:0;pointer-events:none;background-image:url("data:image/svg+xml,%3Csvg width='48' height='48' viewBox='0 0 48 48' xmlns='http://www.w3.org/2000/svg'%3E%3Cpath d='M48 0H0V48' fill='none' stroke='%2364748B' stroke-opacity='0.07'/%3E%3C/svg%3E");mask-image:linear-gradient(180deg,#000 0%,#000 55%,transparent 100%);-webkit-mask-image:linear-gradient(180deg,#000 0%,#000 55%,transparent 100%)}
.bg-glow{position:fixed;border-radius:50%;filter:blur(90px);z-index:0;pointer-events:none}
.g1{width:560px;height:560px;top:-210px;left:-170px;background:radial-gradient(circle,rgba(34,64,153,.5),transparent 70%)}
.g2{width:440px;height:440px;top:-170px;right:-120px;background:radial-gradient(circle,rgba(13,166,237,.30),transparent 70%)}
.page{position:relative;z-index:1;max-width:1440px;margin:0 auto;padding:0 64px}
header{display:flex;align-items:center;justify-content:space-between;height:80px;border-bottom:1px solid var(--border)}
.brand{display:flex;align-items:center;gap:12px}
.brand-icon{width:34px;height:34px;flex:none}
.brand-name{font:800 20px var(--font-num);letter-spacing:1px}
.brand-sub{font-size:11px;color:var(--text3);margin-top:2px}
.hdr-right{display:flex;align-items:center;gap:20px}
.hdr-meta{font-size:12px;color:var(--text2)}
.pill{display:inline-flex;align-items:center;gap:8px;padding:8px 14px;border-radius:999px;font-size:12px;font-weight:600}
.pill.green{background:rgba(52,211,153,.12);border:1px solid rgba(52,211,153,.5);color:var(--green-soft)}
.pill .dot{width:8px;height:8px;border-radius:50%;background:var(--green)}
.gh-link{display:flex;align-items:center;justify-content:center;width:34px;height:34px;flex:none;border-radius:10px;color:var(--text2);border:1px solid var(--border-soft);background:var(--surface);transition:color .2s,border-color .2s,transform .2s}
.gh-link:hover{color:var(--text1);border-color:rgba(34,211,238,.45)}
.gh-link:active{transform:scale(.94)}
.gh-link svg{width:17px;height:17px;display:block}
.hero{padding:56px 0 48px;display:flex;flex-direction:column;gap:28px}
.hero-top{display:flex;justify-content:space-between;align-items:flex-end;gap:24px}
.hero-left{display:flex;flex-direction:column;gap:10px}
.kicker{display:flex;align-items:center;gap:12px}
.tag{display:inline-flex;align-items:center;padding:4px 10px;border-radius:6px;background:var(--surface2);border:1px solid rgba(34,211,238,.4);font-size:11px;font-weight:600;color:var(--cyan-soft)}
.kicker-line{font-size:12px;color:var(--text3)}
h1{font-size:44px;font-weight:700;letter-spacing:.5px}
.hero-sub{font-size:15px;color:var(--text2);max-width:640px;line-height:1.7}
.hero-side{display:flex;flex-direction:column;align-items:flex-end;gap:10px;flex:none}
.hero-pill{margin-top:0}
.hero-pill .pill{padding:6px 12px}
.plain-tip{display:flex;align-items:flex-start;gap:10px;max-width:640px;margin-top:14px;padding:12px 16px;border-radius:12px;background:rgba(52,211,153,.07);border:1px solid rgba(52,211,153,.28);font-size:12px;line-height:1.7;color:var(--text2);flex-wrap:wrap}
.plain-tip .pt-label{flex:none;margin-top:1px;padding:1px 8px;border-radius:999px;background:rgba(52,211,153,.15);border:1px solid rgba(52,211,153,.4);font-size:10px;font-weight:700;color:var(--green-soft);align-self:flex-start}
.plain-tip .pt-body{flex:1;min-width:240px}
.plain-tip strong{color:var(--text1);white-space:nowrap;word-break:keep-all}
.plain-tip a{color:var(--cyan);text-decoration:none;border-bottom:1px dashed rgba(34,211,238,.4)}
.plain-tip a:hover{color:var(--cyan-soft);border-bottom-color:var(--cyan-soft)}
.terms{margin-bottom:16px;border-radius:12px;border:1px solid var(--border-soft);background:rgba(18,26,43,.4);overflow:hidden}
.terms summary{cursor:pointer;padding:12px 16px;font-size:12px;font-weight:600;color:var(--cyan-soft);list-style:none;user-select:none;display:flex;align-items:center;gap:8px}
.terms summary::-webkit-details-marker{display:none}
.terms summary:hover{background:rgba(34,211,238,.04)}
.terms summary .arrow{display:inline-block;color:var(--cyan);font-size:10px;transition:transform .2s}
.terms[open] summary .arrow{transform:rotate(90deg)}
.terms summary .q{display:inline-flex;align-items:center;justify-content:center;width:18px;height:18px;border-radius:50%;background:rgba(34,211,238,.12);border:1px solid rgba(34,211,238,.4);font-size:11px;font-weight:700;color:var(--cyan-soft)}
.terms[open] summary{border-bottom:1px solid var(--border-soft)}
.terms dl{margin:0;padding:6px 16px 14px}
.terms dt{font-size:12px;font-weight:700;color:var(--text1);margin-top:10px}
.terms dd{font-size:12px;line-height:1.7;color:var(--text2);margin:2px 0 0}
.tabs{position:relative}
.tabs>input{position:absolute;opacity:0;pointer-events:none}
.tab-bar{display:flex;flex-wrap:wrap;gap:8px;margin:20px 0 28px;border-bottom:1px solid var(--border);padding-bottom:14px}
.tab-btn{display:inline-flex;align-items:center;gap:8px;padding:9px 18px;border-radius:999px;border:1px solid var(--border-soft);background:var(--surface);color:var(--text2);font-size:13px;font-weight:600;cursor:pointer;user-select:none;transition:color .2s,border-color .2s,background .2s}
.tab-btn .tab-ico{display:inline-flex;align-items:center;justify-content:center;width:22px;height:22px;border-radius:7px;font:700 10px var(--font-num);background:var(--surface2);border:1px solid var(--border-soft);color:var(--text3)}
.tab-btn:hover{color:var(--text1);border-color:rgba(34,211,238,.4)}
#tab-3d:checked~.tab-bar .tab-btn[for="tab-3d"]{color:var(--cyan-soft);border-color:rgba(34,211,238,.5);background:rgba(23,71,97,.5)}
#tab-3d:checked~.tab-bar .tab-btn[for="tab-3d"] .tab-ico{color:var(--cyan-soft);border-color:rgba(34,211,238,.5)}
#tab-ssq:checked~.tab-bar .tab-btn[for="tab-ssq"]{color:var(--violet-soft);border-color:rgba(167,139,250,.5);background:rgba(66,43,112,.5)}
#tab-ssq:checked~.tab-bar .tab-btn[for="tab-ssq"] .tab-ico{color:var(--violet-soft);border-color:rgba(167,139,250,.5)}
.tab-pane{display:none}
#tab-3d:checked~#pane-3d{display:block}
#tab-ssq:checked~#pane-ssq{display:block}
.ssq-kill{margin-top:20px;padding:18px 20px;border-radius:16px;background:linear-gradient(180deg,rgba(18,26,43,.9),rgba(13,20,41,1));border:1px solid rgba(167,139,250,.35)}
.sk-head{display:flex;justify-content:space-between;align-items:center;margin-bottom:14px}
.sk-title{font-size:14px;font-weight:700}
.sk-strategy{font-size:11px;color:var(--text3)}
.sk-body{display:flex;flex-direction:column;gap:10px}
.sk-group{display:flex;align-items:center;gap:12px}
.sk-label{flex:none;font-size:12px;color:var(--text2);display:inline-flex;align-items:center;gap:6px}
.sk-label::before{content:"";width:8px;height:8px;border-radius:50%;flex:none}
.sk-label.rd::before{background:#F87171}
.sk-label.bd::before{background:#60A5FA}
.sk-nums{display:flex;gap:8px;flex-wrap:wrap}
.nb{display:inline-flex;align-items:center;justify-content:center;min-width:34px;height:34px;border-radius:9px;font:700 15px var(--font-num)}
.nb.rd{background:rgba(248,113,113,.12);border:1px solid rgba(248,113,113,.5);color:var(--red)}
.nb.bd{background:rgba(96,165,250,.12);border:1px solid rgba(96,165,250,.5);color:#93C5FD}
.sk-note{margin-top:14px;padding:10px 12px;border-radius:10px;background:rgba(251,146,60,.08);border:1px solid rgba(251,146,60,.35);font-size:11px;line-height:1.7;color:#FDBA74}
.sk-note b{color:#FED7AA}
.ssq-grid{display:grid;grid-template-columns:repeat(2,1fr);gap:14px}
.ssq-card{padding:16px 18px;border-radius:14px;background:var(--surface);border:1px solid var(--border-soft)}
.ssq-hd{display:flex;justify-content:space-between;align-items:center;margin-bottom:12px}
.ssq-hd>span:first-child{font-size:13px;font-weight:600}
.rank-row{display:flex;align-items:center;gap:8px;padding:4px 0}
.rank-num{flex:none;width:26px;font:600 12px var(--font-num);color:var(--text2);text-align:right}
.rank-bar{flex:1;height:14px;border-radius:4px;background:rgba(148,163,184,.12);overflow:hidden}
.rank-fill{height:100%;border-radius:4px}
.rank-fill.hot{background:linear-gradient(90deg,#1D9E75,#34D399)}
.rank-fill.cold{background:linear-gradient(90deg,#185FA5,#60A5FA)}
.rank-fill.miss{background:linear-gradient(90deg,#854F0B,#FBBF24)}
.rank-freq{flex:none;width:44px;font-size:11px;color:var(--text3);text-align:right}
.blue-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:8px}
.bf{display:flex;flex-direction:column;align-items:center;gap:2px;padding:8px 0;border-radius:8px;background:var(--surface2);border:1px solid var(--border-soft)}
.bf b{font:600 13px var(--font-num);color:#93C5FD}
.bf i{font-size:10px;font-style:normal;color:var(--text3)}
.bt-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:12px}
.ssq-hero-card{display:flex;justify-content:space-between;align-items:center;gap:20px;padding:22px 26px;border-radius:var(--radius);background:var(--surface);border:1px solid rgba(52,211,153,.4);box-shadow:0 14px 30px -8px rgba(0,0,0,.35);margin-top:20px}
.shc-left{display:flex;flex-direction:column;gap:6px}
.shc-label{font-size:13px;font-weight:600;color:var(--green-soft)}
.shc-value{font:800 46px var(--font-num);color:var(--green);line-height:1.1}
.shc-sub{font-size:12px;color:var(--text3)}
.shc-meta{font-size:12px;color:var(--text2)}
.shc-meta b{color:var(--green-soft)}
.shc-ring{flex:none}
.ssq-keep{margin-top:14px;padding:16px 20px;border-radius:14px;background:var(--surface);border:1px solid var(--border-soft)}
.keep-grid{display:grid;grid-template-columns:repeat(9,1fr);gap:6px;margin:12px 0}
.keep-num{display:flex;align-items:center;justify-content:center;height:28px;border-radius:7px;background:rgba(52,211,153,.08);border:1px solid rgba(52,211,153,.25);font:600 12px var(--font-num);color:var(--green-soft)}
.disclaimer{margin-top:10px;font-size:10px;color:var(--text3);line-height:1.6}
.ssq-blue{display:inline-flex;align-items:center;justify-content:center;min-width:24px;padding:1px 6px;margin-left:4px;border-radius:6px;background:rgba(96,165,250,.12);border:1px solid rgba(96,165,250,.5);font:700 12px var(--font-num);color:#93C5FD;vertical-align:1px}
.m-detail.ssq-detail .win{font:600 12px var(--font-num)}
.issue-badge{display:flex;flex-direction:column;align-items:center;gap:6px;padding:16px 24px;border-radius:14px;background:var(--surface);border:1px solid rgba(34,211,238,.35);box-shadow:0 0 24px rgba(13,166,237,.15)}
.issue-label{font-size:12px;color:var(--text2)}
.issue-value{font:800 30px var(--font-num);color:var(--cyan-soft)}
.pred-row{display:grid;grid-template-columns:repeat(3,1fr);gap:20px}
.pred-card{display:flex;flex-direction:column;gap:14px;padding:22px 26px;border-radius:var(--radius);border:1px solid var(--border-soft);box-shadow:0 16px 32px -8px rgba(0,0,0,.4);position:relative;overflow:hidden}
.pred-card::before{content:"";position:absolute;inset:0;opacity:.9;z-index:0}
.pred-card>*{position:relative;z-index:1}
.pred-card.cyan{background:linear-gradient(180deg,rgba(23,71,97,.9),rgba(13,20,41,1));border-color:rgba(34,211,238,.35)}
.pred-card.violet{background:linear-gradient(180deg,rgba(66,43,112,.9),rgba(13,20,41,1));border-color:rgba(167,139,250,.35)}
.pred-card.amber{background:linear-gradient(180deg,rgba(107,71,23,.9),rgba(13,20,41,1));border-color:rgba(251,191,36,.35)}
.pred-head{display:flex;justify-content:space-between;align-items:center}
.pred-label{font-size:13px;font-weight:600;color:var(--cyan-soft)}
.pred-card.violet .pred-label{color:var(--violet-soft)}
.pred-card.amber .pred-label{color:var(--amber-soft)}
.mini-tag{padding:4px 8px;border-radius:5px;font-size:10px;font-weight:600;background:rgba(23,71,97,.9);border:1px solid rgba(34,211,238,.4);color:var(--cyan-soft)}
.pred-card.violet .mini-tag{background:rgba(66,43,112,.9);border-color:rgba(167,139,250,.4);color:var(--violet-soft)}
.pred-card.amber .mini-tag{background:rgba(107,71,23,.9);border-color:rgba(251,191,36,.4);color:var(--amber-soft)}
.pred-num{font:800 54px/1.1 var(--font-num);letter-spacing:2px;text-align:center;color:var(--cyan)}
.pred-card.violet .pred-num{color:var(--violet)}
.pred-card.amber .pred-num{color:var(--amber)}
.pred-foot{font-size:11px;color:var(--text3);text-align:center}
.section{padding:0 0 40px}
.section-head{display:flex;justify-content:space-between;align-items:flex-end;margin-bottom:16px}
.section-title{font-size:22px;font-weight:700}
.section-meta{font-size:12px;color:var(--text3)}
.section-note{margin-top:4px;font-size:12px;color:var(--text3);line-height:1.6}
.bento{display:grid;grid-template-columns:340px 1fr;gap:20px}
.big-card{display:flex;flex-direction:column;gap:14px;padding:24px;border-radius:var(--radius);background:var(--surface);border:1px solid rgba(52,211,153,.4);box-shadow:0 14px 30px -8px rgba(0,0,0,.35)}
.big-top{display:flex;justify-content:space-between;align-items:center}
.big-label{font-size:14px;font-weight:600;color:var(--green-soft)}
.ring{flex:none;display:block}
.big-value{font:800 46px var(--font-num);color:var(--green)}
.big-sub{font-size:12px;color:var(--text3)}
.bar{height:6px;border-radius:3px;background:var(--border);overflow:hidden}
.bar-fill{height:100%;border-radius:3px;background:linear-gradient(90deg,var(--green),var(--cyan))}
.grid-6{display:grid;grid-template-columns:repeat(3,1fr);gap:16px}
.stat-card{display:flex;flex-direction:column;justify-content:space-between;gap:8px;padding:18px 20px;border-radius:14px;background:var(--surface);border:1px solid var(--border-soft)}
.stat-top{display:flex;justify-content:space-between;align-items:center}
.stat-label{font-size:12px;color:var(--text2)}
.stat-badge{font-size:10px;font-weight:600}
.stat-value{font:800 30px var(--font-num)}
.engine-row{display:grid;grid-template-columns:1fr 1fr 1fr;gap:20px}
.eng-card{display:flex;flex-direction:column;gap:14px;padding:24px 26px;border-radius:var(--radius);background:var(--surface);border:1px solid var(--border-soft)}
.eng-head{display:flex;align-items:center;gap:10px}
.eng-num{width:26px;height:26px;border-radius:8px;flex:none;display:flex;align-items:center;justify-content:center;font:700 14px var(--font-num);background:var(--surface2);border:1px solid rgba(34,211,238,.4);color:var(--cyan-soft)}
.eng-card.violet .eng-num{border-color:rgba(167,139,250,.4);color:var(--violet-soft)}
.eng-card.amber .eng-num{border-color:rgba(251,191,36,.4);color:var(--amber-soft)}
.eng-title{font-size:15px;font-weight:600}
.eng-desc{font-size:13px;line-height:1.8;color:var(--text2)}
.formula-row{display:flex;align-items:center;gap:14px}
.formula-label{width:40px;font-size:12px;color:var(--text3);flex:none}
.formula{font:600 15px var(--font-num);color:var(--violet)}
.cmp-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}
.wf-note{margin-top:10px;padding:8px 10px;border-radius:8px;background:rgba(52,211,153,.08);border:1px solid rgba(52,211,153,.3);font-size:10px;line-height:1.6;color:var(--text2)}
.trend-card{padding:14px 16px 6px;border-radius:14px;background:var(--surface);border:1px solid var(--border-soft)}
.trend-card svg{display:block;width:100%;height:auto}
.cmp-cell{display:flex;flex-direction:column;gap:4px;padding:12px 14px;border-radius:10px;background:var(--surface2);border:1px solid var(--border-soft)}
.cmp-value{font:800 20px var(--font-num)}
.cmp-label{font-size:10px;color:var(--text3)}
.warn{display:flex;align-items:center;gap:12px;padding:18px 22px;margin-top:40px;border-radius:12px;background:rgba(251,146,60,.08);border:1px solid rgba(251,146,60,.4);font-size:13px;color:#FDBA74;line-height:1.7}
.warn-icon{width:22px;height:22px;border-radius:50%;flex:none;display:flex;align-items:center;justify-content:center;background:rgba(251,191,36,.14);border:1.5px solid rgba(251,191,36,.6);font:700 13px var(--font-num);color:var(--amber)}
.data-alert{display:flex;flex-direction:column;gap:6px;padding:16px 22px;border-radius:12px;margin-bottom:24px;background:linear-gradient(135deg,rgba(230,81,0,.85),rgba(245,124,0,.8));border:1px solid rgba(251,146,60,.5);font-size:13px;line-height:1.7;color:#FED7AA}
.data-alert .da-title{font-size:15px;font-weight:800;color:#FFEDD5}
.table-wrap{border-radius:14px;border:1px solid var(--border);overflow:hidden;background:rgba(18,26,43,.6)}
table{width:100%;border-collapse:collapse;font-size:13px}
thead th{background:var(--surface2);padding:13px 20px;font-size:11px;font-weight:600;color:var(--text3);text-align:center}
tbody td{padding:14px 20px;text-align:center;border-top:1px solid rgba(30,41,59,.6);color:var(--text2)}
tbody tr:nth-child(odd){background:rgba(13,20,41,.5)}
.issue-no{font:400 13px var(--font-num);color:var(--text2)}
.date{font-size:12px;color:var(--text3)}
.win-num{font:700 15px var(--font-num);color:var(--text1)}
.kill-code{font:600 14px var(--font-num)}
.kill-code.ok{color:var(--green)}
.kill-code.bad{color:var(--red)}
.result{display:inline-block;padding:3px 10px;border-radius:999px;font-size:12px;font-weight:700}
.result.ok{background:rgba(52,211,153,.12);border:1px solid rgba(52,211,153,.5);color:var(--green)}
.result.bad{background:rgba(248,113,113,.12);border:1px solid rgba(248,113,113,.5);color:var(--red)}
.m-detail{display:none;flex-direction:column;gap:12px}
.m-card{display:flex;align-items:center;gap:12px;padding:14px 16px;border-radius:12px;background:var(--surface);border:1px solid var(--border)}
.m-card .left{display:flex;flex-direction:column;gap:3px}
.m-card .issue-no{font-size:13px}
.m-card .date{font-size:10px}
.m-card .win{flex:1;text-align:center;font:700 18px var(--font-num);color:var(--text1)}
.v-cyan{color:var(--cyan)}.v-blue{color:var(--blue)}.v-violet{color:var(--violet)}.v-green{color:var(--green)}.v-amber{color:var(--amber)}.v-text2{color:var(--text2)}
.v-cyan-soft{color:var(--cyan-soft)}.v-blue-soft{color:var(--blue-soft)}.v-violet-soft{color:var(--violet-soft)}.v-green-soft{color:var(--green-soft)}
footer{display:flex;flex-direction:column;align-items:center;gap:14px;padding:56px 0 44px;margin-top:16px;border-top:1px solid var(--border)}
.foot-text{font-size:12px;color:var(--text3)}
.foot-meta{font-size:11px;color:#475569}
.foot-brand{font-size:10px;color:#334155}
.pat-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:20px}
.pat-card{display:flex;flex-direction:column;gap:14px;padding:22px 24px;border-radius:var(--radius);background:var(--surface);border:1px solid var(--border-soft)}
.pat-card.danma{border-color:rgba(34,211,238,.35)}
.pat-card.dudan{border-color:rgba(248,113,113,.35)}
.pat-card.sumtail{border-color:rgba(251,191,36,.35)}
.pat-head{display:flex;align-items:center;justify-content:space-between}
.pat-title{font-size:15px;font-weight:700}
.pat-tag{font-size:10px;color:var(--text3);padding:3px 8px;border-radius:999px;background:var(--surface2);border:1px solid var(--border-soft)}
.pat-nums{display:flex;gap:10px}
.pat-num{width:52px;height:52px;border-radius:14px;flex:none;display:flex;align-items:center;justify-content:center;font:800 24px var(--font-num);background:var(--surface2);border:1px solid var(--border-soft)}
.pat-card.danma .pat-num{color:var(--cyan-soft);border-color:rgba(34,211,238,.4)}
.pat-card.dudan .pat-num{color:var(--red);border-color:rgba(248,113,113,.4)}
.pat-card.sumtail .pat-num{color:var(--amber-soft);border-color:rgba(251,191,36,.4)}
.pat-empty{font-size:12px;color:var(--text3)}
.pat-meta{display:flex;flex-direction:column;gap:6px;font-size:12px;color:var(--text2);line-height:1.6}
.pat-meta b{color:var(--green-soft)}
.pat-meta .base{color:var(--text3)}
.pat-detail{font-size:11px;color:var(--text3);line-height:1.8;max-height:150px;overflow-y:auto;scrollbar-width:thin;scrollbar-color:rgba(148,163,184,.35) transparent}
.pat-detail summary{cursor:pointer;color:var(--text2);margin-bottom:4px}
.pat-detail .d-row{display:flex;justify-content:space-between;gap:10px;padding:4px 2px;border-top:1px dashed var(--border-soft)}
.pat-detail .d-path{font:400 12px var(--font-num);color:var(--cyan-soft)}
.pat-detail .d-num{font:600 12px var(--font-num);color:var(--violet-soft);white-space:nowrap}
.sum-grid{display:flex;flex-direction:column;gap:10px}
.sum-row{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:10px 12px;border-radius:10px;background:var(--surface2);border:1px solid var(--border-soft)}
.sum-label{font-size:12px;color:var(--text2)}
.sum-val{font:700 20px var(--font-num);color:var(--amber-soft)}
.sum-meta{font-size:10px;color:var(--text3);text-align:right;line-height:1.5}
@media (max-width:1023px){
.page{padding:0 20px}
header{height:60px}
.hdr-meta{display:none}
.hdr-right{gap:10px}
.gh-link{width:30px;height:30px;border-radius:9px}
.brand-name{font-size:16px}
.brand-sub{display:none}
.brand-icon{width:24px;height:24px}
.pill{padding:6px 10px;font-size:10px}
.hero{padding:28px 0 24px;gap:16px}
.hero-top{align-items:stretch}
.hero-side{align-items:flex-start}
.hero-top{flex-direction:column;align-items:stretch;gap:16px}
h1{font-size:26px}
.hero-sub{font-size:13px;line-height:1.6;max-width:none}
.issue-badge{display:none}
.issue-value{font-size:16px}
.pred-row{display:none}
.pred-grid{display:grid;grid-template-columns:1fr;gap:16px}
.section-head{flex-direction:column;align-items:stretch;gap:6px}
.pred-main{display:flex;flex-direction:column;gap:14px;padding:20px 18px;border-radius:16px;background:linear-gradient(180deg,rgba(23,71,97,.9),rgba(13,20,41,1));border:1px solid rgba(34,211,238,.35);box-shadow:0 14px 28px -8px rgba(0,0,0,.4)}
.pos-row{display:grid;grid-template-columns:repeat(3,1fr);gap:10px}
.pos-card{display:flex;flex-direction:column;align-items:center;gap:6px;padding:12px 10px;border-radius:12px;background:rgba(18,26,43,.8);border:1px solid var(--border-soft)}
.pos-label{font-size:10px;color:var(--text2)}
.pos-num{font:800 26px var(--font-num)}
.section{padding-bottom:24px}
.section-title{font-size:18px}
.section-note{font-size:11px}
.bento{grid-template-columns:1fr}
.big-card{flex-direction:row;align-items:center;padding:16px;gap:16px}
.big-value{font-size:32px}
.big-top{flex-direction:column;align-items:flex-start;gap:2px}
.ring{width:72px;height:72px}
.grid-6{grid-template-columns:repeat(2,1fr);gap:10px}
.stat-card{padding:12px 14px;border-radius:12px}
.stat-value{font-size:22px}
.engine-row{grid-template-columns:1fr;gap:12px}
.eng-card{padding:16px}
.eng-desc{font-size:11px;line-height:1.7}
.formula{font-size:13px}
.cmp-cell{padding:10px 12px}
.warn{margin-top:24px;padding:14px 16px;font-size:11px;line-height:1.6}
.plain-tip{font-size:11px;padding:10px 12px}
.terms summary{font-size:11px}
.terms dt,.terms dd{font-size:11px}
.tab-bar{margin:14px 0 20px;padding-bottom:10px;gap:6px}
.tab-btn{padding:8px 12px;font-size:12px;gap:6px}
.tab-btn .tab-ico{width:20px;height:20px}
.ssq-grid{grid-template-columns:1fr}
.bt-grid{grid-template-columns:1fr}
.pat-grid{grid-template-columns:1fr}
.pat-card{padding:18px 16px}
.pat-num{width:44px;height:44px;font-size:20px}
.ssq-hero-card{flex-direction:column;align-items:flex-start;gap:14px;padding:16px 18px}
.shc-value{font-size:36px}
.keep-grid{grid-template-columns:repeat(6,1fr)}
.blue-grid{grid-template-columns:repeat(8,1fr)}
.bf{padding:6px 0}
.ssq-kill{padding:14px 14px}
.sk-head{flex-direction:column;align-items:flex-start;gap:4px}
.sk-group{align-items:flex-start;flex-direction:column;gap:8px}
.nb{min-width:30px;height:30px;font-size:13px}
.table-wrap{display:none}
.m-detail{display:flex;max-height:62vh;overflow-y:auto;-webkit-overflow-scrolling:touch;overscroll-behavior:contain;padding:2px 8px 2px 2px;border-radius:14px;border:1px solid var(--border);background:rgba(18,26,43,.4);scrollbar-width:thin;scrollbar-color:rgba(148,163,184,.35) transparent}
.m-detail::-webkit-scrollbar{width:6px}
.m-detail::-webkit-scrollbar-track{background:transparent}
.m-detail::-webkit-scrollbar-thumb{background:rgba(148,163,184,.35);border-radius:3px}
.m-detail::-webkit-scrollbar-thumb:hover{background:rgba(148,163,184,.6)}
footer{padding:22px 0 10px;gap:8px}
.foot-text{font-size:10px}
}
@media (min-width:1024px){.pred-grid{display:none}}
{{.MultiCSS}}
</style>
</head>
<body>
<div class="bg-glow g1"></div>
<div class="bg-glow g2"></div>
<div class="bg-grid"></div>

<div class="page">
  <header>
    <div class="brand">
      <svg class="brand-icon" viewBox="0 0 34 34" fill="none" xmlns="http://www.w3.org/2000/svg"><rect x="1" y="1" width="32" height="32" rx="9" stroke="#22D3EE" stroke-width="2"/><circle cx="17" cy="17" r="7" stroke="#22D3EE" stroke-width="2"/><circle cx="17" cy="17" r="2.5" fill="#22D3EE"/><path d="M17 2v6M17 26v6M2 17h6M26 17h6" stroke="#22D3EE" stroke-width="2" stroke-linecap="round"/></svg>
      <div>
        <div class="brand-name">KILL6</div>
        <div class="brand-sub">福彩3D + 双色球 · 数据参考</div>
      </div>
    </div>
    <div class="hdr-right">
      <span class="hdr-meta">福彩3D + 双色球 · 每日自动更新</span>
      <a class="gh-link" href="https://github.com/wu529778790/fc3d-kill6" target="_blank" rel="noopener noreferrer" aria-label="GitHub 仓库"><svg viewBox="0 0 16 16" fill="currentColor" xmlns="http://www.w3.org/2000/svg"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z"/></svg></a>
    </div>
  </header>

  <div class="tabs">
    <input type="radio" name="lot" id="tab-3d" checked>
    <input type="radio" name="lot" id="tab-ssq">
    {{.MultiBar}}
    <nav class="tab-bar">
      <label class="tab-btn" for="tab-3d"><span class="tab-ico">3D</span>福彩3D</label>
      <label class="tab-btn" for="tab-ssq"><span class="tab-ico">球</span>双色球</label>
      {{.MultiBtns}}
    </nav>
    <div class="tab-pane" id="pane-3d">
  <main>
{{if .Banners.DataFailed}}
<div class="data-alert"><div class="da-title">数据源异常</div>所有数据源获取失败，页面为最后一次成功数据，请检查数据源（灰鸟 / 17500.cn）。</div>
{{end}}

    <section class="hero">
      <div class="hero-top">
        <div class="hero-left">
          <div class="kicker">
            <span class="tag">V9.3 六杀制</span>
            <span class="kicker-line">双引擎独立杀码 · kill1 决策树 + kill2 算术公式</span>
          </div>
          <h1>福彩3D 百十个杀码参考</h1>
          <p class="hero-sub">每天开奖一个三位数。我们提前排除 6 个号码（百位 2 个、十位 2 个、个位 2 个），开奖号一个都没被排除 = 全中。近 100 期全中率 {{printf "%.1f" .Meta.Period6Pct100}}%，远超"闭眼随便排除"的 51.2%。</p>
          <div class="plain-tip"><span class="pt-label">小白版</span><span class="pt-body">「杀码」= <strong>排除掉的号码</strong>；「6 杀全中」= 排除的 6 个号码一个都没开出来。下面三框里的数字，就是今天要排除的 6 个号码。</span></div>
        </div>
        <div class="hero-side">
          <div class="hero-pill"><span class="pill green"><span class="dot"></span>福彩3D · 近100期 6杀全中 {{printf "%.1f" .Meta.Period6Pct100}}%</span></div>
          <div class="issue-badge">
            <span class="issue-label">下一期参考 · 第 {{.NextIssue}} 期</span>
            <span class="issue-value">6 杀码</span>
          </div>
        </div>
      </div>

      <div class="pred-row">
        <div class="pred-card cyan">
          <div class="pred-head"><span class="pred-label">百位 · 双杀码</span><span class="mini-tag">kill1 + kill2</span></div>
          <div class="pred-num">{{.Pred.H}}, {{.Pred.H2}}</div>
          <div class="pred-foot">kill1 决策树 + 算术公式 · 近{{.Meta.BacktestN}}期命中 {{printf "%.1f" .Meta.AccH}}%</div>
        </div>
        <div class="pred-card violet">
          <div class="pred-head"><span class="pred-label">十位 · 双杀码</span><span class="mini-tag">kill1 + kill2</span></div>
          <div class="pred-num">{{.Pred.T}}, {{.Pred.T2}}</div>
          <div class="pred-foot">kill1 决策树 + 算术公式 · 近{{.Meta.BacktestN}}期命中 {{printf "%.1f" .Meta.AccT}}%</div>
        </div>
        <div class="pred-card amber">
          <div class="pred-head"><span class="pred-label">个位 · 双杀码</span><span class="mini-tag">kill1 + kill2</span></div>
          <div class="pred-num">{{.Pred.O}}, {{.Pred.O2}}</div>
          <div class="pred-foot">kill1 决策树 + 算术公式 · 近{{.Meta.BacktestN}}期命中 {{printf "%.1f" .Meta.AccO}}%</div>
        </div>
      </div>

      <div class="pred-grid">
        <div class="pred-main">
          <div class="pred-head"><span class="pred-label">下一期参考 · 第 {{.NextIssue}} 期</span><span class="mini-tag">kill1 + kill2</span></div>
          <div class="pos-row">
            <div class="pos-card"><span class="pos-label">百位</span><span class="pos-num v-cyan">{{.Pred.H}}, {{.Pred.H2}}</span></div>
            <div class="pos-card"><span class="pos-label">十位</span><span class="pos-num v-violet">{{.Pred.T}}, {{.Pred.T2}}</span></div>
            <div class="pos-card"><span class="pos-label">个位</span><span class="pos-num v-amber">{{.Pred.O}}, {{.Pred.O2}}</span></div>
          </div>
        </div>
      </div>
    </section>

    <section class="section">
      <div class="section-head">
        <h2 class="section-title">近 {{.Meta.BacktestN}} 期回测</h2>
        <span class="section-meta">3杀综合 {{printf "%.1f" .Meta.AccAll}}% · 6杀全中 {{printf "%.1f" .Meta.Period6Pct100}}% · 随机基线 51.2%</span>
        <p class="section-note">人话：近 100 天里，有 {{printf "%.1f" .Meta.Period6Pct100}}% 的天数我们把 6 个号码全排对了（闭眼乱排只有 51.2%）。</p>
      </div>
      <div class="bento">
        <div class="big-card">
          <div class="big-top">
            <span class="big-label">6 杀全中率</span>
            {{.Ring}}
          </div>
          <div class="big-value">{{printf "%.1f" .Meta.Period6Pct100}}%</div>
          <div class="big-sub">近 100 期 · 超越随机基线 +{{printf "%.1f" .Pct6Beat}}pp</div>
          <div class="bar"><div class="bar-fill" style="width:{{printf "%.0f" .Meta.Period6Pct100}}%"></div></div>
        </div>
        <div class="grid-6">
          <div class="stat-card"><div class="stat-top"><span class="stat-label">百位 · 3杀</span><span class="stat-badge v-cyan-soft">错 {{.Meta.ErrH}}</span></div><div class="stat-value v-cyan">{{printf "%.1f" .Meta.AccH}}%</div></div>
          <div class="stat-card"><div class="stat-top"><span class="stat-label">十位 · 3杀</span><span class="stat-badge v-blue-soft">错 {{.Meta.ErrT}}</span></div><div class="stat-value v-blue">{{printf "%.1f" .Meta.AccT}}%</div></div>
          <div class="stat-card"><div class="stat-top"><span class="stat-label">个位 · 3杀</span><span class="stat-badge v-violet-soft">错 {{.Meta.ErrO}}</span></div><div class="stat-value v-violet">{{printf "%.1f" .Meta.AccO}}%</div></div>
          <div class="stat-card"><div class="stat-top"><span class="stat-label">百位 · kill2</span><span class="stat-badge v-blue-soft">错 {{.Meta.ErrH2}}</span></div><div class="stat-value v-blue">{{printf "%.1f" .Meta.AccH2}}%</div></div>
          <div class="stat-card"><div class="stat-top"><span class="stat-label">十位 · kill2</span><span class="stat-badge v-violet-soft">错 {{.Meta.ErrT2}}</span></div><div class="stat-value v-violet">{{printf "%.1f" .Meta.AccT2}}%</div></div>
          <div class="stat-card"><div class="stat-top"><span class="stat-label">个位 · kill2</span><span class="stat-badge v-green-soft">错 {{.Meta.ErrO2}}</span></div><div class="stat-value v-green">{{printf "%.1f" .Meta.AccO2}}%</div></div>
        </div>
      </div>
    </section>

    <section class="section">
      <div class="section-head">
        <h2 class="section-title">V9 六杀引擎</h2>
        <span class="section-meta">kill1 决策树 + kill2 算术公式 · 双引擎独立运算、互为校验</span>
        <p class="section-note">人话：两套算法互不搭伙、独立计算，两边都算同一个号码，才把它列进排除名单。</p>
      </div>
      <div class="engine-row">
        <div class="eng-card">
          <div class="eng-head"><span class="eng-num">1</span><span class="eng-title">kill1 · V8 条件决策树</span></div>
          <div class="eng-desc">百位：10 条件决策树，逐期推导最优杀码。<br>十位：V8a 公式法，历经两处漏洞修复。<br>个位：12 条件决策树 + 自适应备份（5 期失败窗口自动切换）。</div>
        </div>
        <div class="eng-card violet">
          <div class="eng-head"><span class="eng-num">2</span><span class="eng-title">kill2 · 独立算术公式</span></div>
          <div class="formula-row"><span class="formula-label">百位</span><span class="formula">(b − span + 9) mod 10</span></div>
          <div class="formula-row"><span class="formula-label">十位</span><span class="formula">(s − mid + 5) mod 10</span></div>
          <div class="formula-row"><span class="formula-label">个位</span><span class="formula">(g² + |b − g|) mod 10</span></div>
        </div>
        <div class="eng-card amber">
          <div class="eng-head"><span class="eng-num">3</span><span class="eng-title">6杀全中 · 数据对比</span></div>
          <div class="cmp-grid">
            <div class="cmp-cell"><span class="cmp-value v-green">{{printf "%.1f" .Meta.Period6Pct100}}%</span><span class="cmp-label">近 100 期</span></div>
            <div class="cmp-cell"><span class="cmp-value v-blue">≈53%</span><span class="cmp-label">全量 8730 期收敛</span></div>
            <div class="cmp-cell"><span class="cmp-value v-violet">≈66%</span><span class="cmp-label">6杀全中理论上限</span></div>
            <div class="cmp-cell"><span class="cmp-value v-text2">51.2%</span><span class="cmp-label">随机基线</span></div>
          </div>
          {{if .WFNote}}<div class="wf-note">{{.WFNote}}</div>{{end}}
        </div>
      </div>
      <div class="warn">
        <span class="warn-icon">!</span>
        <span>理性参考提示：彩票本质是随机游戏，杀码结果仅基于历史数据统计，不构成任何投注建议。请理性娱乐。</span>
      </div>
    </section>

    {{if .TrendSVG}}
    <section class="section">
      <div class="section-head">
        <h2 class="section-title">6 杀率趋势</h2>
        <span class="section-meta">滚动 100 期 · 每日自动记录 · 虚线为 51.2% 随机基线</span>
        <p class="section-note">人话：曲线往上 = 最近排得更准；曲线持续下行 = 前期高光期滑出窗口，属正常回落。</p>
      </div>
      <div class="trend-card">
        <svg viewBox="0 0 600 200" role="img" aria-label="6杀全中率趋势折线图">{{.TrendSVG}}</svg>
      </div>
    </section>
    {{end}}

    {{if .Pat}}
    <section class="section">
      <div class="section-head">
        <h2 class="section-title">规律挖掘 · 跨期组合参考</h2>
        <span class="section-meta">胆码 / 毒胆 / 和尾杀号 · 算法移植自 lottery-analyzer · 近{{.Pat.Window}}期滚动回测</span>
        <p class="section-note">人话：把历史按期号切成块，挖「连续多块都成立的跨期数字组合」，套到最新几期给出参考。与杀码引擎相互独立，如实对照随机基线，不构成投注建议。</p>
      </div>
      <div class="pat-grid">
        <div class="pat-card danma">
          <div class="pat-head"><span class="pat-title">胆码参考</span><span class="pat-tag">至少一位开出</span></div>
          <div class="pat-nums">{{range .Pat.Danma.Picks}}<span class="pat-num">{{.}}</span>{{end}}{{if not .Pat.Danma.Picks}}<span class="pat-empty">暂无稳定规律</span>{{end}}</div>
          <div class="pat-meta"><span>近{{.Pat.Window}}期命中 <b>{{printf "%.1f" .Pat.DanmaSt.Rate}}%</b>（随机基线 {{printf "%.1f" .Pat.DanmaSt.Base}}%）</span><span>全量 {{.Pat.DanmaSt.FullN}} 期 {{printf "%.1f" .Pat.DanmaSt.FullRate}}%</span></div>
          <details class="pat-detail"><summary>规律明细 · {{.Pat.Danma.HitCount}} 条</summary>{{range .Pat.Danma.Hits}}<div class="d-row"><span class="d-path">{{.Path}}</span><span class="d-num">连中{{.MaxCons}}块 · {{joinNums .Next}}</span></div>{{end}}</details>
        </div>
        <div class="pat-card dudan">
          <div class="pat-head"><span class="pat-title">毒胆参考</span><span class="pat-tag">全部不开</span></div>
          <div class="pat-nums">{{range .Pat.Dudan.Picks}}<span class="pat-num">{{.}}</span>{{end}}{{if not .Pat.Dudan.Picks}}<span class="pat-empty">暂无稳定规律</span>{{end}}</div>
          <div class="pat-meta"><span>近{{.Pat.Window}}期命中 <b>{{printf "%.1f" .Pat.DudanSt.Rate}}%</b>（随机基线 {{printf "%.1f" .Pat.DudanSt.Base}}%）</span><span>全量 {{.Pat.DudanSt.FullN}} 期 {{printf "%.1f" .Pat.DudanSt.FullRate}}%</span></div>
          <details class="pat-detail"><summary>规律明细 · {{.Pat.Dudan.HitCount}} 条</summary>{{range .Pat.Dudan.Hits}}<div class="d-row"><span class="d-path">{{.Path}}</span><span class="d-num">连中{{.MaxCons}}块 · {{joinNums .Next}}</span></div>{{end}}</details>
        </div>
        <div class="pat-card sumtail">
          <div class="pat-head"><span class="pat-title">和尾杀号</span><span class="pat-tag">组合和尾 ≠ 位和尾</span></div>
          <div class="sum-grid">
            <div class="sum-row"><span class="sum-label">百个和尾 · 杀</span><span class="sum-val">{{if .Pat.SumBH.Picks}}{{index .Pat.SumBH.Picks 0}}{{else}}—{{end}}</span><span class="sum-meta">近{{.Pat.Window}}期 {{printf "%.1f" .Pat.SumBHSt.Rate}}%<br>基线 {{printf "%.1f" .Pat.SumBHSt.Base}}%</span></div>
            <div class="sum-row"><span class="sum-label">百十和尾 · 杀</span><span class="sum-val">{{if .Pat.SumBT.Picks}}{{index .Pat.SumBT.Picks 0}}{{else}}—{{end}}</span><span class="sum-meta">近{{.Pat.Window}}期 {{printf "%.1f" .Pat.SumBTSt.Rate}}%<br>基线 {{printf "%.1f" .Pat.SumBTSt.Base}}%</span></div>
            <div class="sum-row"><span class="sum-label">十个和尾 · 杀</span><span class="sum-val">{{if .Pat.SumTO.Picks}}{{index .Pat.SumTO.Picks 0}}{{else}}—{{end}}</span><span class="sum-meta">近{{.Pat.Window}}期 {{printf "%.1f" .Pat.SumTOSt.Rate}}%<br>基线 {{printf "%.1f" .Pat.SumTOSt.Base}}%</span></div>
          </div>
          <div class="pat-meta"><span>和尾 = 开奖号三位相加的个位。杀和尾 = 排除「下期和尾 = 该数字」的组合规律，基线为 90%。</span></div>
        </div>
      </div>
    </section>
    {{end}}

    <section class="section">
      <div class="section-head">
        <h2 class="section-title">近 {{.Meta.BacktestN}} 期回测明细</h2>
        <span class="section-meta">完整 {{.Meta.BacktestN}} 期滚动 · 移动端卡片式</span>
        <p class="section-note">人话：每一行 = 一天。「全中」= 当天排除的 6 个号码一个都没开出来。</p>
      </div>
      <details class="terms" id="terms">
        <summary><span class="arrow">▸</span><span class="q">?</span>彩票小白？点这里展开 7 个术语解释</summary>
        <dl>
          <dt>杀码 / 杀</dt><dd>排除掉的号码。「杀」= 排除，是我们对「不会开出来」的判断。</dd>
          <dt>百位 / 十位 / 个位</dt><dd>三位号码的三个位置。例如开奖号 380：3 是百位、8 是十位、0 是个位。</dd>
          <dt>双杀码</dt><dd>每个位置排除 2 个号码，三个位置共排除 6 个号码。</dd>
          <dt>6 杀全中</dt><dd>排除的 6 个号码一个都没开出来 = 这期算得准，叫「全中」。</dd>
          <dt>期号</dt><dd>每天的期次编号，例如 2026222 代表 2026 年第 222 期。</dd>
          <dt>随机基线</dt><dd>闭着眼随便排除 6 个号码，理论上能全中的概率是 51.2%。页面上所有成绩都是跟它比。</dd>
          <dt>回测</dt><dd>拿过去 100 天的真实开奖数据，模拟「每天提前算好、再对答案」，验证算法到底准不准。</dd>
        </dl>
      </details>
      <div class="table-wrap">
        <table>
          <thead><tr><th>期号</th><th>日期</th><th>开奖</th><th>百位杀码</th><th>十位杀码</th><th>个位杀码</th><th>6杀结果</th></tr></thead>
          <tbody>
{{range .Rows}}
<tr>
<td class="issue-no">{{.Issue}}</td><td class="date">{{.Date}}</td><td class="win-num">{{.Open}}</td>
<td class="kill-code {{if and .HOK .H2OK}}ok{{else}}bad{{end}}">{{.HK}}, {{.HK2}}</td>
<td class="kill-code {{if and .TOK .T2OK}}ok{{else}}bad{{end}}">{{.TK}}, {{.TK2}}</td>
<td class="kill-code {{if and .OOK .O2OK}}ok{{else}}bad{{end}}">{{.OK}}, {{.OK2}}</td>
<td><span class="result {{if .All6OK}}ok{{else}}bad{{end}}">{{if .All6OK}}全中{{else}}未中{{end}}</span></td>
</tr>
{{end}}
          </tbody>
        </table>
      </div>
      <div class="m-detail">
{{range .MCards}}
<div class="m-card"><div class="left"><span class="issue-no">{{.Issue}}</span><span class="date">{{.Date}}</span></div><span class="win">{{.Open}}</span><span class="result {{if .All6OK}}ok{{else}}bad{{end}}">{{if .All6OK}}全中{{else}}未中{{end}}</span></div>
{{end}}
      </div>
    </section>
  </main>
    </div>
    <div class="tab-pane" id="pane-ssq">
  <main>
    <section class="hero">
      <div class="hero-top">
        <div class="hero-left">
          <div class="kicker">
            <span class="tag">数据参考</span>
            <span class="kicker-line">红球 1-33 选 6 · 蓝球 1-16 选 1</span>
          </div>
          <h1>双色球 杀号参考</h1>
          <p class="hero-sub">每期开 6 个红球 + 1 个蓝球。蓝球只有 16 个，每期排除 3 个——<strong>10 次有 8 次避开</strong>，这是数学上的结构性优势；红球空间大，我们如实展示、不吹。</p>
          <div class="plain-tip"><span class="pt-label">小白版</span><span class="pt-body">红球 = 红色号码区（1-33 里开 6 个），蓝球 = 蓝色号码区（1-16 里开 1 个）。「杀号」= <strong>帮你排除掉</strong>的号码。</span></div>
        </div>
        <div class="issue-badge">
          <span class="issue-label">最新开奖 · 第 {{.SSQ.Meta.LatestIssue}} 期</span>
          <span class="issue-value">{{.SSQ.Meta.LatestDate}}</span>
        </div>
      </div>
      <div class="ssq-hero-card">
        <div class="shc-left">
          <span class="shc-label">杀蓝避开率 · 全量回测</span>
          <div class="shc-value">{{printf "%.1f" .SSQ.Meta.BluePct}}%</div>
          <div class="shc-sub">蓝球 16 选 1 · 每期排除 3 个</div>
          <div class="shc-meta">最近 100 期 <b>{{printf "%.1f" .SSQ.Meta.RecentBluePct}}%</b> · 随机基线 {{printf "%.1f" .SSQ.Meta.BaseBlue}}%</div>
        </div>
        <div class="shc-ring">{{.SSQRing}}</div>
      </div>
      <div class="ssq-kill">
        <div class="sk-head"><span class="sk-title">下期参考 · 第 {{.SSQ.Meta.NextIssue}} 期</span><span class="sk-strategy">{{.SSQ.Meta.Strategy}} · 近{{.SSQ.Meta.Window}}期统计</span></div>
        <div class="sk-body">
          <div class="sk-group"><span class="sk-label rd">杀红球 · 6 个</span><span class="sk-nums">{{range .SSQ.Meta.KillReds}}<b class="nb rd">{{printf "%02d" .}}</b>{{end}}</span></div>
          <div class="sk-group"><span class="sk-label bd">杀蓝球 · 3 个</span><span class="sk-nums">{{range .SSQ.Meta.KillBlues}}<b class="nb bd">{{printf "%02d" .}}</b>{{end}}</span></div>
        </div>
        <div class="sk-note">全量 {{.SSQ.Meta.Total}} 期回测：杀蓝避开 <b>{{printf "%.1f" .SSQ.Meta.BluePct}}%</b>（基线 {{printf "%.1f" .SSQ.Meta.BaseBlue}}%——蓝球仅 16 个，排除 3 个天然 8 成避开）· 杀红全避开 <b>{{printf "%.1f" .SSQ.Meta.RedPct}}%</b>（基线 {{printf "%.1f" .SSQ.Meta.BaseRed}}%）。红球空间大，如实对照基线。</div>
      </div>
      <div class="ssq-keep">
        <div class="sk-head"><span class="sk-title">本期红球保留池</span><span class="sk-strategy">{{len .SSQ.KeepReds}} 个 · 33 减 6</span></div>
        <div class="keep-grid">{{range .SSQ.KeepReds}}<span class="keep-num">{{printf "%02d" .}}</span>{{end}}</div>
        <div class="sk-note">保留池 = 33 个红球去掉排除的 6 个。红球命中与随机相当（全量 {{printf "%.1f" .SSQ.Meta.RedPct}}% vs 基线 {{printf "%.1f" .SSQ.Meta.BaseRed}}%），仅供参考。</div>
      </div>
      <p class="disclaimer">红球组合空间巨大（33 选 6 ≈ 110 万种），历史统计无法稳定预判红球号码；本页数据仅供选号参考，不构成投注建议，请理性娱乐。</p>
    </section>

    <section class="section">
      <div class="section-head">
        <h2 class="section-title">冷热 · 遗漏 · 蓝球频率</h2>
        <span class="section-meta">近 20 期统计</span>
        <p class="section-note">人话：哪个红球最近常出（热）、最近少出（冷）、最久没出（遗漏）——仅供参考，不构成出号依据。</p>
      </div>
      <div class="ssq-grid">
        <div class="ssq-card"><div class="ssq-hd"><span>红球热号 Top6</span><span class="mini-tag">近20期</span></div>
{{range $i, $r := .SSQ.HotReds}}{{if lt $i 6}}<div class="rank-row"><span class="rank-num">{{printf "%02d" $r.Num}}</span><div class="rank-bar"><div class="rank-fill hot" style="width:{{pctW $r.Freq $.SSQ.MaxFreq}}%"></div></div><span class="rank-freq">{{$r.Freq}}次</span></div>{{end}}{{end}}</div>
        <div class="ssq-card"><div class="ssq-hd"><span>红球冷号 Top6</span><span class="mini-tag">近20期</span></div>
{{range $i, $r := .SSQ.ColdReds}}{{if lt $i 6}}<div class="rank-row"><span class="rank-num">{{printf "%02d" $r.Num}}</span><div class="rank-bar"><div class="rank-fill cold" style="width:{{pctW $r.Freq $.SSQ.MaxFreq}}%"></div></div><span class="rank-freq">{{$r.Freq}}次</span></div>{{end}}{{end}}</div>
        <div class="ssq-card"><div class="ssq-hd"><span>红球遗漏 Top6</span><span class="mini-tag">最久没出</span></div>
{{range $i, $r := .SSQ.MissReds}}{{if lt $i 6}}<div class="rank-row"><span class="rank-num">{{printf "%02d" $r.Num}}</span><div class="rank-bar"><div class="rank-fill miss" style="width:{{pctW $r.Miss $.SSQ.MissMax}}%"></div></div><span class="rank-freq">{{$r.Miss}}期</span></div>{{end}}{{end}}</div>
        <div class="ssq-card"><div class="ssq-hd"><span>蓝球频率</span><span class="mini-tag">近20期</span></div>
          <div class="blue-grid">{{range .SSQ.BlueFreq}}<span class="bf"><b>{{printf "%02d" .Num}}</b><i>{{.Freq}}</i></span>{{end}}</div>
        </div>
      </div>
    </section>

    <section class="section">
      <div class="section-head">
        <h2 class="section-title">红球和值走势</h2>
        <span class="section-meta">近 20 期 · 均值 {{.SSQ.SumAvg}}</span>
        <p class="section-note">人话：和值 = 6 个红球号码相加。双色球红球和值长期围绕均值 102 波动，偏离太大就是冷门组合。</p>
      </div>
      <div class="trend-card"><svg viewBox="0 0 600 200" role="img" aria-label="红球和值走势">{{ssqSumSVG .SSQ.SumTrend .SSQ.SumAvg}}</svg></div>
    </section>

    <section class="section">
      <div class="section-head">
        <h2 class="section-title">回测：策略 vs 随机基线</h2>
        <span class="section-meta">{{.SSQ.Meta.Strategy}} · 近{{.SSQ.Meta.Window}}期窗口</span>
        <p class="section-note">人话：把杀号策略拿到过去每一期"提前算好、再对答案"，看命中率和闭眼乱排差多少——结论是基本一样。</p>
      </div>
      <div class="bt-grid">
        <div class="cmp-cell"><span class="cmp-value v-green">{{printf "%.1f" .SSQ.Meta.BluePct}}%</span><span class="cmp-label">杀蓝避开（基线 {{printf "%.1f" .SSQ.Meta.BaseBlue}}%）</span></div>
        <div class="cmp-cell"><span class="cmp-value v-text2">{{printf "%.1f" .SSQ.Meta.RedPct}}%</span><span class="cmp-label">杀红全避开（基线 {{printf "%.1f" .SSQ.Meta.BaseRed}}%）</span></div>
        <div class="cmp-cell"><span class="cmp-value v-text2">{{printf "%.1f" .SSQ.Meta.AllPct}}%</span><span class="cmp-label">红蓝全避开（基线 {{printf "%.1f" .SSQ.Meta.BaseAll}}%）</span></div>
      </div>
      <div class="wf-note">最近 100 期：杀蓝避开 <b>{{printf "%.1f" .SSQ.Meta.RecentBluePct}}%</b> · 杀红 <b>{{printf "%.1f" .SSQ.Meta.RecentRedPct}}%</b> · 全中 <b>{{printf "%.1f" .SSQ.Meta.RecentAllPct}}%</b>（对应基线 {{printf "%.1f" .SSQ.Meta.BaseAll}}%）</div>
      <div class="wf-note">Walk-forward 滚动验证{{range .SSQ.Meta.WF}} · 近{{.Label}}全中 {{printf "%.1f" .All6Pct}}%（p={{pf .PVal}}）{{end}}——与随机基线无显著差异（p 值均不显著），如实呈现。</div>
    </section>

    <section class="section">
      <div class="section-head">
        <h2 class="section-title">近 100 期回测明细</h2>
        <span class="section-meta">完整 100 期滚动 · 移动端卡片式</span>
        <p class="section-note">人话：每一行 = 一期。「全中」= 当期排除的 6 红 + 3 蓝一个都没开出来。</p>
      </div>
      <div class="table-wrap">
        <table>
          <thead><tr><th>期号</th><th>日期</th><th>开奖</th><th>杀红</th><th>杀蓝</th><th>结果</th></tr></thead>
          <tbody>
{{range .SSQ.Meta.Rows}}
<tr>
<td class="issue-no">{{.Issue}}</td><td class="date">{{.Date}}</td>
<td class="win-num">{{ssqReds .Reds}} <span class="ssq-blue">{{printf "%02d" .Blue}}</span></td>
<td class="kill-code {{if .RedOK}}ok{{else}}bad{{end}}">{{ssqKills .KillReds}}</td>
<td class="kill-code {{if .BlueOK}}ok{{else}}bad{{end}}">{{ssqKills .KillBlues}}</td>
<td><span class="result {{if .AllOK}}ok{{else}}bad{{end}}">{{if .AllOK}}全中{{else}}未中{{end}}</span></td>
</tr>
{{end}}
          </tbody>
        </table>
      </div>
      <div class="m-detail ssq-detail">
{{range .SSQ.Meta.Rows}}
<div class="m-card"><div class="left"><span class="issue-no">{{.Issue}}</span><span class="date">{{.Date}}</span></div><span class="win">{{ssqReds .Reds}} · 蓝{{printf "%02d" .Blue}}</span><span class="result {{if .AllOK}}ok{{else}}bad{{end}}">{{if .AllOK}}全中{{else}}未中{{end}}</span></div>
{{end}}
      </div>
    </section>

    <details class="terms" id="terms-ssq">
      <summary><span class="arrow">▸</span><span class="q">?</span>双色球术语表</summary>
      <dl>
        <dt>红球 / 蓝球</dt><dd>红球区 1-33 每期开 6 个；蓝球区 1-16 每期开 1 个。红球 + 蓝球构成一注。</dd>
        <dt>杀号 / 排除</dt><dd>从号码里去掉我们认为"不太会开"的。双色球里数学上做不到准确，仅供缩小选号范围。</dd>
        <dt>和值</dt><dd>6 个红球号码相加的总和，长期围绕均值 102 波动。</dd>
        <dt>热号 / 冷号</dt><dd>近 20 期出现次数多 / 少的号码。</dd>
        <dt>遗漏</dt><dd>某个号码距离上次开出隔了多少期。</dd>
        <dt>随机基线</dt><dd>闭眼随便排除同样数量的号码，理论上能全中的概率。所有成绩都跟它比。</dd>
      </dl>
    </details>
  </main>
    </div>
    {{.MultiPanes}}
  </div>

  <footer>
    <span class="foot-text">数据来源：福彩3D / 双色球 历史开奖数据 · 算法严格不含未来信息 · 仅供研究参考</span>
    <span class="foot-meta">3D 数据截止 {{.Meta.LatestDate}}{{if .SSQ}} · 双色球截止 {{.SSQ.Meta.LatestDate}}{{end}} · 每日开奖后自动更新</span>
    <span class="foot-brand">KILL6 · V9.3 六杀制引擎 + 双色球统计工具</span>
  </footer>
</div>
<script>
// wx-auth-sdk 可选认证（jsdmirror 主源 + jsdelivr 兜底，加载失败静默不影响页面）。
// 不锁版本（@latest）：SDK 侧更新后本页自动跟进，无需改代码。
// 1.2.36+ 为小程序扫码登录，打开弹窗即出码，全程无需输入验证码；
// 已认证用户（7 天滑动 Cookie）静默通过，不弹窗。
(function () {
  var CDNS = [
    'https://cdn.jsdmirror.com/npm/wx-auth-sdk@latest/dist/wx-auth.umd.js',
    'https://cdn.jsdelivr.net/npm/wx-auth-sdk@latest/dist/wx-auth.umd.js'
  ];
  var css = document.createElement('link');
  css.rel = 'stylesheet';
  css.href = CDNS[0].replace('wx-auth.umd.js', 'wx-auth.css');
  document.head.appendChild(css);
  function load(i) {
    if (i >= CDNS.length) return;
    var s = document.createElement('script');
    s.src = CDNS[i];
    s.onload = function () {
      if (window.WxAuth) {
        WxAuth.init({
          required: false,
          onVerified: function (u) { console.log('[wx-auth] 认证成功', u); }
        });
      }
    };
    s.onerror = function () { load(i + 1); };
    document.body.appendChild(s);
  }
  load(0);
})();
</script>
</body>
</html>`

// GenerateHTML 渲染完整页面（Rows 自动转为最新在前；mv 为多彩种页签，可为 nil）
func GenerateHTML(m backtest.Meta, pred backtest.Predict, rows []backtest.Row, b Banners, nextIssue string, wf []backtest.WFWindow, sv *SSQView, pr *pattern.BacktestResult, mv *MultiViews) (string, error) {
	funcs := template.FuncMap{
		"pctW": func(v, max int) int {
			if max <= 0 {
				return 0
			}
			p := v * 100 / max
			if p > 100 {
				p = 100
			}
			return p
		},
		"pf": func(p float64) string {
			if p < 0.001 {
				return "<0.001"
			}
			return fmt.Sprintf("%.3f", p)
		},
		"joinNums": func(a []int) string {
			parts := make([]string, len(a))
			for i, n := range a {
				parts[i] = fmt.Sprintf("%d", n)
			}
			return strings.Join(parts, ",")
		},
		"ssqSumSVG": ssqSumSVG,
		"ssqReds": func(rs [6]int) string {
			parts := make([]string, 6)
			for i, n := range rs {
				parts[i] = fmt.Sprintf("%02d", n)
			}
			return strings.Join(parts, ",")
		},
		"ssqKills": func(a []int) string {
			parts := make([]string, len(a))
			for i, n := range a {
				parts[i] = fmt.Sprintf("%02d", n)
			}
			return strings.Join(parts, ",")
		},
	}
	t, err := template.New("page").Funcs(funcs).Parse(tmplSrc)
	if err != nil {
		return "", err
	}
	rev := reverseRows(rows)
	mc := rev // 移动端卡片 = 全部 100 期，与 section-meta "完整 100 期滚动" 承诺一致
	data := view{
		Meta: m, Pred: pred,
		Rows: rev, MCards: mc,
		Banners: b, NextIssue: nextIssue,
		Ring:     ringSVG(m.Period6Pct100),
		Pct6Beat: m.Period6Pct100 - 51.2,
		WFNote:   wfNote(wf),
		TrendSVG: trendSVG(rev),
		SSQRing:  "",
		SSQ:      sv,
		Pat:      buildPatView(pr),
	}
	if sv != nil {
		data.SSQRing = ringSVG(sv.Meta.BluePct)
	}
	if mv != nil {
		data.MultiCSS = mv.multiCSS()
		data.MultiBar = mv.multiRadios()
		data.MultiBtns = mv.multiBtns()
		data.MultiPanes = mv.multiPanes()
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// buildPatView 组装规律挖掘区块视图数据
func buildPatView(pr *pattern.BacktestResult) *PatView {
	if pr == nil {
		return nil
	}
	pv := &PatView{Window: pr.Window}
	pv.Danma = pr.Latest[pattern.Danma]
	pv.Dudan = pr.Latest[pattern.Dudan]
	pv.SumBH = pr.Latest[pattern.SumBH]
	pv.SumBT = pr.Latest[pattern.SumBT]
	pv.SumTO = pr.Latest[pattern.SumTO]
	pv.DanmaSt = pr.Stat(pattern.Danma)
	pv.DudanSt = pr.Stat(pattern.Dudan)
	pv.SumBHSt = pr.Stat(pattern.SumBH)
	pv.SumBTSt = pr.Stat(pattern.SumBT)
	pv.SumTOSt = pr.Stat(pattern.SumTO)
	return pv
}

// ssqSumSVG 红球和值走势折线（SVG 片段，含均值虚线）
func ssqSumSVG(trend []int, avg int) string {
	n := len(trend)
	if n == 0 {
		return ""
	}
	const W, H = 600.0, 200.0
	const padL, padR, padT, padB = 10.0, 40.0, 18.0, 24.0
	plotW := W - padL - padR
	plotH := H - padT - padB
	yMin, yMax := 60.0, 145.0
	x := func(i int) float64 { return padL + float64(i)*(plotW/float64(n-1)) }
	y := func(v int) float64 { return padT + (yMax-float64(v))/(yMax-yMin)*plotH }

	pts := make([]string, 0, n)
	for i, v := range trend {
		pts = append(pts, fmt.Sprintf("%.1f,%.1f", x(i), y(v)))
	}
	var sb strings.Builder
	// 均值虚线
	ay := y(avg)
	sb.WriteString(fmt.Sprintf(`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#64748B" stroke-width="1" stroke-dasharray="4 3" opacity="0.5"/>`, padL, ay, W-padR, ay))
	sb.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" fill="#64748B" font-size="10" font-family="'SF Mono',ui-monospace,Menlo,monospace">均值 %d</text>`, W-padR+4, ay+3, avg))
	// 折线
	sb.WriteString(`<polyline points="` + strings.Join(pts, " ") + `" fill="none" stroke="#A78BFA" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>`)
	// 最新点
	lx, ly := x(n-1), y(trend[n-1])
	sb.WriteString(fmt.Sprintf(`<circle cx="%.1f" cy="%.1f" r="3.5" fill="#A78BFA"/>`, lx, ly))
	sb.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" fill="#A78BFA" font-size="13" font-weight="600" font-family="'SF Mono',ui-monospace,Menlo,monospace">%d</text>`, lx-10, ly-10, trend[n-1]))
	return sb.String()
}

// wfNote 生成 walk-forward 摘要行（取 100 期窗口）
func wfNote(wf []backtest.WFWindow) string {
	if len(wf) == 0 {
		return ""
	}
	w := wf[0]
	pv := fmt.Sprintf("%.4f", w.PVal)
	if w.PVal < 0.001 {
		pv = "<0.001"
	}
	return fmt.Sprintf("Walk-forward 滚动验证：近 %d 期 6 杀全中 %.1f%% vs 随机基线 51.2%%，超越 %+.1fpp（z=%.1f, p=%s）——白话：每期都用开奖前已有的数据提前算好、再对答案，不含任何「马后炮」。",
		w.N, w.All6Pct, w.BeatPP, w.Z, pv)
}

// trendSVG 生成 6 杀全中率趋势折线（SVG 片段，含 70% 预警线与 51.2% 基线）
//
// 输入为回测逐期明细：x 轴从最新（左）到最早（右），y 轴为"若从该期起
// 回测到现在的累计 6 杀全中率"。这样可以立刻基于已有 100 期数据画出
// 算法表现轨迹，不必等 monitor 历史积累。
func trendSVG(rows []backtest.Row) string {
	n := len(rows)
	if n == 0 {
		return ""
	}
	const W, H = 600.0, 200.0
	const padL, padR, padT, padB = 10.0, 40.0, 18.0, 24.0
	plotW := W - padL - padR
	plotH := H - padT - padB
	yMin, yMax := 40.0, 95.0
	x := func(i int) float64 { return padL + float64(i)*(plotW/float64(n-1)) }
	y := func(pct float64) float64 { return padT + (yMax-pct)/(yMax-yMin)*plotH }

	// rows 是最新在前：rows[0]=最新, rows[n-1]=最早
	// 从最早往最新累加，得到每个 i 对应"从 i 往窗口末尾"的累计命中率。
	cum := make([]float64, n)
	running := 0
	for i := n - 1; i >= 0; i-- {
		if rows[i].All6OK {
			running++
		}
		cum[i] = float64(running) / float64(n-i) * 100
	}

	pts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		pts = append(pts, fmt.Sprintf("%.1f,%.1f", x(i), y(cum[i])))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<line x1="%.1f" y1="%.1f" x2="%.1f" y2="%.1f" stroke="#64748B" stroke-width="1" stroke-dasharray="4 3" opacity="0.5"/>`, padL, y(51.2), W-padR, y(51.2)))
	sb.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" fill="#64748B" font-size="10" font-family="'SF Mono',ui-monospace,Menlo,monospace">51.2%%</text>`, W-padR+4, y(51.2)+3))
	sb.WriteString(`<polyline points="` + strings.Join(pts, " ") + `" fill="none" stroke="#34D399" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>`)
	// 左端（最新一期）+ 数值标签 + 右端也加圆
	lx, ly := x(0), y(cum[0])
	rx, ry := x(n-1), y(cum[n-1])
	sb.WriteString(fmt.Sprintf(`<circle cx="%.1f" cy="%.1f" r="3.5" fill="#34D399"/>`, lx, ly))
	sb.WriteString(fmt.Sprintf(`<text x="%.1f" y="%.1f" fill="#34D399" font-size="13" font-weight="600" font-family="'SF Mono',ui-monospace,Menlo,monospace">%.1f%%</text>`, lx+6, ly-8, cum[0]))
	sb.WriteString(fmt.Sprintf(`<circle cx="%.1f" cy="%.1f" r="2.5" fill="#64748B"/>`, rx, ry))
	_ = ry
	return sb.String()
}
