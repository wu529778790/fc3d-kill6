// 多彩种开奖数据抓取（灰鸟 API 主源 + 17500.cn / 福彩官网备源）。
// 各彩种双源可用性：
//
//	排列3/排列5/大乐透/快乐8：灰鸟 + 17500 (data.17500.cn/{key}_asc.txt)
//	七乐彩：灰鸟 + 福彩官网 findDrawNotice
//	七星彩：灰鸟单源（体彩官网有反爬、17500 无存档）
package fetch

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"fc3d-kill6/data"
)

// LatestNum 通用彩种最新一期
type LatestNum struct {
	Game      data.GameDef
	Issue     string
	Date      string
	Nums      []int
	NextIssue string
	Source    string
}

// huiniaoType 灰鸟 API 的 type 参数（与 GameDef.Key 不同的在此映射）
var huiniaoType = map[string]string{
	"pl3": "pls", "pl5": "plw", "qxc": "qxc", "dlt": "dlt", "qlc": "qlc",
}

// cwlName 福彩官网 findDrawNotice 的 name 参数（备源）
var cwlName = map[string]string{
	"qlc": "qlc", "kl8": "kl8",
}

// f17500Key 17500 存档文件名前缀
var f17500Key = map[string]string{
	"pl3": "pl3", "pl5": "pl5", "dlt": "dlt", "kl8": "kl8",
}

// FetchLatestNum 依次尝试该彩种的数据源，返回 (新数据, 源是否存活)。
// 期号必须 > 本地 CSV 最新期号，否则视为缓存/旧数据拒绝。
func FetchLatestNum(g data.GameDef, csvPath string) (*LatestNum, bool) {
	lastIssue := ""
	if draws, err := data.LoadNumCSV(csvPath, g); err == nil {
		lastIssue = data.LastNumIssue(draws)
	}

	alive := false
	sources := []struct {
		name string
		fn   func() (*LatestNum, error)
	}{}
	if _, ok := huiniaoType[g.Key]; ok {
		sources = append(sources, struct {
			name string
			fn   func() (*LatestNum, error)
		}{"灰鸟API(" + g.Key + ")", func() (*LatestNum, error) { return fetchHuiniaoNum(g) }})
	}
	if _, ok := f17500Key[g.Key]; ok {
		sources = append(sources, struct {
			name string
			fn   func() (*LatestNum, error)
		}{"17500.cn(" + g.Key + ")", func() (*LatestNum, error) { return fetch17500Num(g) }})
	}
	if _, ok := cwlName[g.Key]; ok {
		sources = append(sources, struct {
			name string
			fn   func() (*LatestNum, error)
		}{"福彩官网(" + g.Key + ")", func() (*LatestNum, error) { return fetchCWLNum(g) }})
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
		if lastIssue != "" && lt.Issue <= lastIssue {
			fmt.Printf("  ⏭️ %s: 期号%s<=本地%s, 跳过(无新期, 源正常)\n", src.name, lt.Issue, lastIssue)
			continue
		}
		fmt.Printf("  ✅ %s: %s (%s) %v\n", src.name, lt.Issue, lt.Date, lt.Nums)
		return lt, true
	}
	return nil, alive
}

// fnum 兼容灰鸟 API 的数字字段（排列3 为 int，排列5/大乐透 等为 "07" 字符串）
type fnum int

func (f *fnum) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		*f = -1
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("非数字字段 %q", s)
	}
	*f = fnum(n)
	return nil
}

type huiniaoItem struct {
	Code     string `json:"code"`
	Day      string `json:"day"`
	One      fnum   `json:"one"`
	Two      fnum   `json:"two"`
	Three    fnum   `json:"three"`
	Four     fnum   `json:"four"`
	Five     fnum   `json:"five"`
	Six      fnum   `json:"six"`
	Seven    fnum   `json:"seven"`
	Eight    fnum   `json:"eight"`
	NextCode string `json:"next_code"`
}

// FetchHuiniaoPageNum 灰鸟 API 分页拉取（新→旧），供全量导入用。
// 返回该页条目（每条 Nums 已按彩种长度截取校验）。
func FetchHuiniaoPageNum(g data.GameDef, page, limit int) ([]data.NumDraw, error) {
	t := huiniaoType[g.Key]
	url := fmt.Sprintf("http://api.huiniao.top/interface/home/lotteryHistory?type=%s&page=%d&limit=%d", t, page, limit)
	body, err := httpGet(url, "")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Data struct {
				List []huiniaoItem `json:"list"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Code != 1 {
		return nil, fmt.Errorf("灰鸟API(%s) 返回异常 code=%d", g.Key, resp.Code)
	}
	out := make([]data.NumDraw, 0, len(resp.Data.Data.List))
	for _, it := range resp.Data.Data.List {
		d, err := huiniaoToDraw(g, it)
		if err != nil {
			continue // 单条坏行跳过
		}
		out = append(out, *d)
	}
	return out, nil
}

func huiniaoToDraw(g data.GameDef, it huiniaoItem) (*data.NumDraw, error) {
	flat := []int{int(it.One), int(it.Two), int(it.Three), int(it.Four), int(it.Five), int(it.Six), int(it.Seven), int(it.Eight)}
	if len(flat) < g.NumLen {
		return nil, fmt.Errorf("字段不足")
	}
	nums := make([]int, g.NumLen)
	copy(nums, flat[:g.NumLen])
	for _, n := range nums {
		if n < g.MinVal || n > g.MaxVal {
			return nil, fmt.Errorf("号码 %d 超界", n)
		}
	}
	return &data.NumDraw{Issue: it.Code, Date: it.Day, Nums: nums}, nil
}

func fetchHuiniaoNum(g data.GameDef) (*LatestNum, error) {
	t := huiniaoType[g.Key]
	url := fmt.Sprintf("http://api.huiniao.top/interface/home/lotteryHistory?type=%s&page=1&limit=1", t)
	body, err := httpGet(url, "")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Last huiniaoItem `json:"last"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Code != 1 || resp.Data.Last.Code == "" {
		return nil, fmt.Errorf("灰鸟API(%s) 返回异常 code=%d", g.Key, resp.Code)
	}
	d, err := huiniaoToDraw(g, resp.Data.Last)
	if err != nil {
		return nil, fmt.Errorf("灰鸟API(%s) %v", g.Key, err)
	}
	return &LatestNum{Game: g, Issue: d.Issue, Date: d.Date, Nums: d.Nums, NextIssue: resp.Data.Last.NextCode, Source: "灰鸟API"}, nil
}

// fetch17500Num 17500 全量 TXT（升序），取行尾最新一期。
// 注意直连 data.17500.cn；www.17500.cn/getData/ 的重定向目标文件名不统一。
func fetch17500Num(g data.GameDef) (*LatestNum, error) {
	url := fmt.Sprintf("https://data.17500.cn/%s_asc.txt", f17500Key[g.Key])
	body, err := httpGetRetry(url)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("17500(%s) 无数据", g.Key)
	}
	fields := strings.Fields(strings.TrimSpace(lines[len(lines)-1]))
	if len(fields) < 2+g.NumLen {
		return nil, fmt.Errorf("17500(%s) 行字段不足: %d", g.Key, len(fields))
	}
	if !regexp.MustCompile(`^\d{5,7}$`).MatchString(fields[0]) {
		return nil, fmt.Errorf("17500(%s) 期号解析失败: %q", g.Key, fields[0])
	}
	nums := make([]int, g.NumLen)
	for i := 0; i < g.NumLen; i++ {
		n, err := strconv.Atoi(fields[2+i])
		if err != nil || n < g.MinVal || n > g.MaxVal {
			return nil, fmt.Errorf("17500(%s) 号码解析失败: %q", g.Key, fields[2+i])
		}
		nums[i] = n
	}
	return &LatestNum{Game: g, Issue: fields[0], Date: fields[1], Nums: nums, Source: "17500.cn"}, nil
}

func httpGetRetry(url string) ([]byte, error) {
	var body []byte
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		body, lastErr = httpGet(url, "")
		if lastErr == nil {
			break
		}
	}
	return body, lastErr
}

// Fetch17500Full 拉取 17500 全量 TXT（升序），解析全部行（供历史导入）。
func Fetch17500Full(g data.GameDef) ([]data.NumDraw, error) {
	url := fmt.Sprintf("https://data.17500.cn/%s_asc.txt", f17500Key[g.Key])
	body, err := httpGetRetry(url)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	out := make([]data.NumDraw, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2+g.NumLen {
			continue
		}
		if !regexp.MustCompile(`^\d{5,7}$`).MatchString(fields[0]) {
			continue
		}
		nums := make([]int, g.NumLen)
		ok := true
		for i := 0; i < g.NumLen; i++ {
			n, err := strconv.Atoi(fields[2+i])
			if err != nil || n < g.MinVal || n > g.MaxVal {
				ok = false
				break
			}
			nums[i] = n
		}
		if !ok {
			continue
		}
		out = append(out, data.NumDraw{Issue: fields[0], Date: fields[1], Nums: nums})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("17500(%s) 全量解析为空", g.Key)
	}
	return out, nil
}

// FetchHuiniaoFull 灰鸟 API 分页拉取全量历史（新→旧分页，返回按时间正序）。
// 拉到空页为止；限流/单页失败重试后跳过（下一页继续），避免短页误判提前终止。
func FetchHuiniaoFull(g data.GameDef, maxPages int) ([]data.NumDraw, error) {
	var all []data.NumDraw
	for page := 1; page <= maxPages; page++ {
		var items []data.NumDraw
		var err error
		for attempt := 0; attempt < 3; attempt++ {
			items, err = FetchHuiniaoPageNum(g, page, 500)
			if err == nil && len(items) > 0 {
				break
			}
			time.Sleep(3 * time.Second) // 限流等待重试
		}
		if err != nil || len(items) == 0 {
			break // 该页确实无数据
		}
		all = append(all, items...)
		fmt.Printf("  %s 灰鸟 page=%d: %d 条 (累计 %d)\n", g.Name, page, len(items), len(all))
		time.Sleep(1 * time.Second) // 页间节流
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("灰鸟API(%s) 全量拉取为空", g.Key)
	}
	// 反转为时间正序
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}
	return all, nil
}

// FetchCWLFull 福彩官网分页拉取全量历史（供七乐彩等备源导入）。
func FetchCWLFull(g data.GameDef, pages int) ([]data.NumDraw, error) {
	name := cwlName[g.Key]
	out := make([]data.NumDraw, 0, 2048)
	for page := 1; page <= pages; page++ {
		url := fmt.Sprintf("https://www.cwl.gov.cn/cwl_admin/front/cwlkj/search/kjxx/findDrawNotice?name=%s&system=0&pageNo=%d&pageSize=100", name, page)
		body, err := httpGetRetry(url)
		if err != nil {
			return nil, err
		}
		var resp struct {
			State  int `json:"state"`
			Result []struct {
				Code string `json:"code"`
				Date string `json:"date"`
				Red  string `json:"red"`
				Blue string `json:"blue"`
			} `json:"result"`
		}
		if err := json.Unmarshal(body, &resp); err != nil || resp.State != 0 {
			break
		}
		if len(resp.Result) == 0 {
			break
		}
		for _, it := range resp.Result {
			date := it.Date
			if i := strings.Index(date, "("); i > 0 {
				date = date[:i]
			}
			parts := strings.Split(it.Red, ",")
			if it.Blue != "" {
				parts = append(parts, it.Blue)
			}
			if len(parts) != g.NumLen {
				continue
			}
			nums := make([]int, g.NumLen)
			ok := true
			for i, p := range parts {
				n, e := strconv.Atoi(strings.TrimSpace(p))
				if e != nil || n < g.MinVal || n > g.MaxVal {
					ok = false
					break
				}
				nums[i] = n
			}
			if !ok {
				continue
			}
			out = append(out, data.NumDraw{Issue: it.Code, Date: date, Nums: nums})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("福彩官网(%s) 全量拉取为空", g.Key)
	}
	// 接口默认新→旧，反转为正序
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// WriteCSV 全量写出彩种 CSV：已有行 + 新行合并、按期号去重并升序排序后原子覆写。
// 保证文件时间有序（回测依赖顺序，乱序会静默污染结果）。
func WriteCSV(path string, g data.GameDef, draws []data.NumDraw) (int, error) {
	merged := make([]data.NumDraw, 0, len(draws)+1024)
	seen := map[string]bool{}
	if old, err := data.LoadNumCSV(path, g); err == nil {
		for _, d := range old {
			if !seen[d.Issue] {
				merged = append(merged, d)
				seen[d.Issue] = true
			}
		}
	}
	added := 0
	for _, d := range draws {
		if !seen[d.Issue] {
			merged = append(merged, d)
			seen[d.Issue] = true
			added++
		}
	}
	sort.SliceStable(merged, func(a, b int) bool {
		na, ea := strconv.Atoi(merged[a].Issue)
		nb, eb := strconv.Atoi(merged[b].Issue)
		if ea != nil || eb != nil {
			return merged[a].Issue < merged[b].Issue
		}
		return na < nb
	})

	hdr := "issue,date"
	for i := 1; i <= g.NumLen; i++ {
		hdr += fmt.Sprintf(",n%d", i)
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return 0, err
	}
	w := csv.NewWriter(f)
	if _, err := f.WriteString(hdr + "\n"); err != nil {
		f.Close()
		return 0, err
	}
	for _, d := range merged {
		row := make([]string, 0, 2+g.NumLen)
		row = append(row, d.Issue, d.Date)
		for _, n := range d.Nums {
			row = append(row, strconv.Itoa(n))
		}
		if err := w.Write(row); err != nil {
			f.Close()
			return added, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		f.Close()
		return added, err
	}
	if err := f.Close(); err != nil {
		return added, err
	}
	return added, os.Rename(tmp, path)
}

// 返回 red=基本号逗号串, blue=特别号（快乐8 无 blue）。
// fetchCWLNum 福彩官网 findDrawNotice 备源（七乐彩/快乐8）。
func fetchCWLNum(g data.GameDef) (*LatestNum, error) {
	name := cwlName[g.Key]
	url := fmt.Sprintf("https://www.cwl.gov.cn/cwl_admin/front/cwlkj/search/kjxx/findDrawNotice?name=%s&issueCount=1", name)
	body, err := httpGetRetry(url)
	if err != nil {
		return nil, err
	}
	var resp struct {
		State  int `json:"state"`
		Result []struct {
			Code string `json:"code"`
			Date string `json:"date"` // 形如 "2026-09-25(五)"
			Red  string `json:"red"`
			Blue string `json:"blue"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.State != 0 || len(resp.Result) == 0 {
		return nil, fmt.Errorf("福彩官网(%s) 返回异常 state=%d", g.Key, resp.State)
	}
	it := resp.Result[0]
	date := it.Date
	if i := strings.Index(date, "("); i > 0 {
		date = date[:i]
	}
	redParts := strings.Split(it.Red, ",")
	parts := redParts
	if it.Blue != "" {
		parts = append(parts, it.Blue)
	}
	if len(parts) != g.NumLen {
		return nil, fmt.Errorf("福彩官网(%s) 号码个数 %d != %d", g.Key, len(parts), g.NumLen)
	}
	nums := make([]int, g.NumLen)
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < g.MinVal || n > g.MaxVal {
			return nil, fmt.Errorf("福彩官网(%s) 号码解析失败: %q", g.Key, p)
		}
		nums[i] = n
	}
	return &LatestNum{Game: g, Issue: it.Code, Date: date, Nums: nums, Source: "福彩官网"}, nil
}
