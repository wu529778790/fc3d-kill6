// 多彩种数据层：通用开奖 CSV 读写（排列3/排列5/七星彩/大乐透/七乐彩/快乐8）。
// 统一格式：issue,date,v1,v2,...（数字型为各位数字，号码型为全部开出号码）。
package data

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// GameDef 彩种定义
type GameDef struct {
	Key    string // 文件名/标识，如 "pl3"
	Name   string // 中文名
	NumLen int    // 每期号码个数
	MaxVal int    // 单个号码最大值（数字型 9，号码型 35/30/80 等）
	MinVal int    // 单个号码最小值（数字型 0，号码型 1）
	Digit  bool   // 是否数字型（可投影为 3 位跑 kill6 引擎）
}

// MultiGames 新增彩种定义（顺序即页面页签顺序）
var MultiGames = []GameDef{
	{Key: "pl3", Name: "排列3", NumLen: 3, MaxVal: 9, MinVal: 0, Digit: true},
	{Key: "pl5", Name: "排列5", NumLen: 5, MaxVal: 9, MinVal: 0, Digit: true},
	{Key: "qxc", Name: "七星彩", NumLen: 7, MaxVal: 9, MinVal: 0, Digit: true},
	{Key: "dlt", Name: "大乐透", NumLen: 7, MaxVal: 35, MinVal: 1, Digit: false},
	{Key: "qlc", Name: "七乐彩", NumLen: 8, MaxVal: 30, MinVal: 1, Digit: false},
	{Key: "kl8", Name: "快乐8", NumLen: 20, MaxVal: 80, MinVal: 1, Digit: false},
}

// GameByKey 按 Key 查彩种定义
func GameByKey(key string) (GameDef, bool) {
	for _, g := range MultiGames {
		if g.Key == key {
			return g, true
		}
	}
	return GameDef{}, false
}

// NumDraw 一期通用开奖记录（排列N 为各位数字；大乐透 5 前 + 2 后；七乐彩 7 基本号 + 1 特别号；快乐8 为 20 个号码）
type NumDraw struct {
	Issue string
	Date  string
	Nums  []int
}

// LoadNumCSV 读取彩种 CSV（issue,date,v1,v2,...），容忍脏行。
func LoadNumCSV(path string, g GameDef) ([]NumDraw, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	draws := make([]NumDraw, 0, 4096)
	first := true
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil || first {
			first = false
			continue
		}
		d, ok := parseNumRow(rec, g)
		if !ok {
			continue
		}
		draws = append(draws, d)
	}
	return draws, nil
}

func parseNumRow(rec []string, g GameDef) (NumDraw, bool) {
	if len(rec) < 2+g.NumLen {
		return NumDraw{}, false
	}
	nums := make([]int, 0, g.NumLen)
	for i := 2; i < 2+g.NumLen; i++ {
		n, err := strconv.Atoi(strings.TrimSpace(rec[i]))
		if err != nil || n < g.MinVal || n > g.MaxVal {
			return NumDraw{}, false
		}
		nums = append(nums, n)
	}
	return NumDraw{Issue: strings.TrimSpace(rec[0]), Date: strings.TrimSpace(rec[1]), Nums: nums}, true
}

// AppendNumCSV 追加一期；期号已存在返回 0（幂等）。
func AppendNumCSV(path string, g GameDef, d NumDraw) (int, error) {
	if len(d.Nums) != g.NumLen {
		return 0, fmt.Errorf("%s 号码个数 %d != %d", g.Key, len(d.Nums), g.NumLen)
	}
	for _, n := range d.Nums {
		if n < g.MinVal || n > g.MaxVal {
			return 0, fmt.Errorf("%s 号码 %d 超出 [%d,%d]", g.Key, n, g.MinVal, g.MaxVal)
		}
	}
	existing := map[string]bool{}
	if draws, err := LoadNumCSV(path, g); err == nil {
		for _, dr := range draws {
			existing[dr.Issue] = true
		}
	}
	if existing[d.Issue] {
		return 0, nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if fi, err := f.Stat(); err == nil && fi.Size() == 0 {
		hdr := "issue,date"
		for i := 1; i <= g.NumLen; i++ {
			hdr += fmt.Sprintf(",n%d", i)
		}
		if _, err := f.WriteString(hdr + "\n"); err != nil {
			return 0, err
		}
	}
	w := csv.NewWriter(f)
	row := make([]string, 0, 2+g.NumLen)
	row = append(row, d.Issue, d.Date)
	for _, n := range d.Nums {
		row = append(row, strconv.Itoa(n))
	}
	if err := w.Write(row); err != nil {
		return 0, err
	}
	w.Flush()
	return 1, w.Error()
}

// LastNumIssue 返回最新期号，空数据返回 ""。
func LastNumIssue(draws []NumDraw) string {
	if len(draws) == 0 {
		return ""
	}
	return draws[len(draws)-1].Issue
}

// Project3 取末三位投影为 Draw（数字型彩种跑 kill6 引擎用）。
// Nums 长度必须 ≥3；返回 B=倒数第3位, S=倒数第2位, G=末位。
func Project3(draws []NumDraw) []Draw {
	out := make([]Draw, len(draws))
	for i, d := range draws {
		n := len(d.Nums)
		out[i] = Draw{Issue: d.Issue, Date: d.Date, B: d.Nums[n-3], S: d.Nums[n-2], G: d.Nums[n-1]}
	}
	return out
}

// OpenString 开奖号展示串（号码型两位补零，数字型原样）
func (d NumDraw) OpenString(g GameDef) string {
	parts := make([]string, len(d.Nums))
	for i, n := range d.Nums {
		if g.Digit {
			parts[i] = strconv.Itoa(n)
		} else {
			parts[i] = fmt.Sprintf("%02d", n)
		}
	}
	return strings.Join(parts, ",")
}
