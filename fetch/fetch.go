// Package fetch 提供开奖数据双源抓取（灰鸟 API 主 + 17500.cn 备份）、
// 期号合理性校验与跨年期号计算。
package fetch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"fc3d-kill6/data"
)

// Latest 从数据源获取到的最新一期
type Latest struct {
	Issue     string
	Date      string
	B, S, G   int
	NextIssue string // 数据源自带的下期期号（跨年安全），可为空
	Source    string
}

// FetchLatest 依次尝试数据源，返回 (新数据, 源是否存活)。
// 期号必须 > 本地 CSV 最新期号，否则视为缓存/旧数据拒绝（防假数据）。
// alive=true 表示至少一个源成功返回数据（即使无新期，属开奖前正常）。
func FetchLatest(csvPath string) (*Latest, bool) {
	lastIssue := ""
	if draws, err := data.LoadCSV(csvPath); err == nil {
		lastIssue = data.LastIssue(draws)
	}

	alive := false
	sources := []struct {
		name string
		fn   func() (*Latest, error)
	}{
		{"灰鸟API", fetchHuiniao},
		{"17500.cn", fetch17500},
	}
	for _, src := range sources {
		lt, err := src.fn()
		if err != nil || lt == nil {
			if err != nil {
				fmt.Printf("  ⚠️ %s: %v\n", src.name, err)
			}
			continue
		}
		alive = true
		// 期号合理性校验
		if lastIssue != "" && lt.Issue <= lastIssue {
			fmt.Printf("  ⏭️ %s: 期号%s<=本地%s, 跳过(无新期, 源正常)\n", src.name, lt.Issue, lastIssue)
			continue
		}
		fmt.Printf("  ✅ %s: %s (%s) %d%d%d\n", src.name, lt.Issue, lt.Date, lt.B, lt.S, lt.G)
		return lt, true
	}
	return nil, alive
}

// 灰鸟 API：自带 next_code 字段，天然支持跨年期号回绕
func fetchHuiniao() (*Latest, error) {
	const url = "http://api.huiniao.top/interface/home/lotteryHistory?type=fcsd&page=1&limit=1"
	body, err := httpGet(url, "")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Data struct {
				List []struct {
					Code     string `json:"code"`
					Day      string `json:"day"`
					One      int    `json:"one"`
					Two      int    `json:"two"`
					Three    int    `json:"three"`
					NextCode string `json:"next_code"`
				} `json:"list"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Code != 1 || len(resp.Data.Data.List) == 0 {
		return nil, fmt.Errorf("灰鸟API 返回异常 code=%d", resp.Code)
	}
	it := resp.Data.Data.List[0]
	return &Latest{
		Issue: it.Code, Date: it.Day,
		B: it.One, S: it.Two, G: it.Three,
		NextIssue: it.NextCode, Source: "灰鸟API",
	}, nil
}

// 17500.cn：官方级全量 TXT（每行: 期号 日期 百 十 个 [附加列...]），带 429 重试。
// 直连 data.17500.cn 的升序存档：www.17500.cn/getData/ 只会 302 到该文件，
// 且 2026-11 起行尾附加列（试机号/销售额等）从 15 列变为 17 列，定宽截取会解析错位。
func fetch17500() (*Latest, error) {
	body, err := fetch17500Body()
	if err != nil {
		return nil, err
	}
	return parse17500Line(body)
}

const h17500URL = "https://data.17500.cn/3d_asc.txt"

var h17500UAs = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36",
	"Python-urllib/3.11",
}

// fetch17500Body 下载 17500 全量 TXT，带 429 限流重试（换 UA 重试 3 次）。
func fetch17500Body() ([]byte, error) {
	var body []byte
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		body, lastErr = httpGet(h17500URL, h17500UAs[attempt%len(h17500UAs)])
		if lastErr == nil {
			return body, nil
		}
		time.Sleep(2 * time.Second)
	}
	return nil, lastErr
}

// Parse17500All 解析 17500 全量 TXT 的全部合法数据行（按期号升序）。
func Parse17500All(body []byte) []data.Draw {
	out := make([]data.Draw, 0, 8192)
	for _, line := range strings.Split(string(body), "\n") {
		f := strings.Fields(line)
		if len(f) < 5 {
			continue
		}
		lt, err := latestFromLast5(f[0], f[1], f[2], f[3], f[4])
		if err != nil {
			continue
		}
		out = append(out, data.Draw{Issue: lt.Issue, Date: lt.Date, B: lt.B, S: lt.S, G: lt.G})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Issue < out[j].Issue })
	return out
}

// FetchMissing 补历史缺口：抓数源中断一天就漏一期，而正常抓数只补最新一期，
// 缺口会永久留在 CSV 里。这里下载 17500 全量 TXT，返回期号大于本地最新期的所有期（升序），
// 由调用方按期号顺序追加，保证 CSV 不被追加成乱序。
func FetchMissing(csvPath string) ([]data.Draw, error) {
	var last string
	if draws, err := data.LoadCSV(csvPath); err == nil {
		last = data.LastIssue(draws)
	}
	body, err := fetch17500Body()
	if err != nil {
		return nil, err
	}
	return missingAfter(Parse17500All(body), last), nil
}

// missingAfter 返回期号严格大于 last 的记录（入参已升序）。
func missingAfter(all []data.Draw, last string) []data.Draw {
	out := []data.Draw{}
	for _, d := range all {
		if d.Issue > last {
			out = append(out, d)
		}
	}
	return out
}

// errTooFew 17500 数据字段不足
var errTooFew = fmt.Errorf("17500 数据字段不足")

// parse17500Line 解析 17500 全量 TXT：逐行从末尾往前找第一条合法数据行，取其前 5 列
// （期号 日期 百 十 个）。行尾附加列数量会随数据源改版变化（历史 15 列、2026-11 起 17 列），
// 因此不做定宽截取，避免期号错位成号码。
func parse17500Line(body []byte) (*Latest, error) {
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		fields := strings.Fields(lines[i])
		if len(fields) < 5 {
			continue
		}
		lt, err := latestFromLast5(fields[0], fields[1], fields[2], fields[3], fields[4])
		if err != nil {
			continue // 表头/脏行跳过
		}
		return lt, nil
	}
	return nil, errTooFew
}

func latestFromLast5(issue, date, bs, ss, gs string) (*Latest, error) {
	if !regexp.MustCompile(`^20\d{5}$`).MatchString(issue) {
		return nil, fmt.Errorf("17500 行尾期号解析失败: %q", issue)
	}
	b, e1 := strconv.Atoi(bs)
	s, e2 := strconv.Atoi(ss)
	g, e3 := strconv.Atoi(gs)
	if e1 != nil || e2 != nil || e3 != nil {
		return nil, fmt.Errorf("17500 数字解析失败")
	}
	return &Latest{Issue: issue, Date: date, B: b, S: s, G: g, Source: "17500.cn"}, nil
}

func httpGet(url, ua string) ([]byte, error) {
	if ua == "" {
		ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}

// NextIssueCalc 计算下一期期号，跨年安全（对齐 next_issue_calc）：
// 1) 数据源 next_issue 优先（天然含跨年回绕）
// 2) 兜底按开奖日期 12-31 → 次年 001
// 3) 最后兜底 year+seq+1（补零）
func NextIssueCalc(issue, date, nextIssue string) string {
	if nextIssue != "" {
		return nextIssue
	}
	if len(issue) >= 7 {
		yy, err := strconv.Atoi(issue[:4])
		seq, err2 := strconv.Atoi(issue[4:])
		if err == nil && err2 == nil {
			if d, err3 := time.Parse("2006-01-02", date); err3 == nil {
				if d.Month() == 12 && d.Day() == 31 {
					return fmt.Sprintf("%d001", yy+1)
				}
			}
			return fmt.Sprintf("%d%03d", yy, seq+1)
		}
	}
	return ""
}
