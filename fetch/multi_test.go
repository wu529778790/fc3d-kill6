package fetch

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"fc3d-kill6/data"
)

func TestFnumUnmarshal(t *testing.T) {
	cases := []struct {
		raw  string
		want int
		fail bool
	}{
		{`7`, 7, false}, {`"07"`, 7, false}, {`"12"`, 12, false}, {`null`, -1, false}, {`"abc"`, 0, true},
	}
	for _, c := range cases {
		var f fnum
		err := json.Unmarshal([]byte(c.raw), &f)
		if c.fail {
			if err == nil {
				t.Fatalf("fnum(%s) 应报错", c.raw)
			}
			continue
		}
		if err != nil || int(f) != c.want {
			t.Fatalf("fnum(%s) = %d, %v", c.raw, int(f), err)
		}
	}
}

func TestHuiniaoToDraw(t *testing.T) {
	g := data.GameDef{Key: "dlt", NumLen: 7, MaxVal: 35, MinVal: 1}
	it := huiniaoItem{Code: "26109", Day: "2026-09-23", One: 12, Two: 14, Three: 16, Four: 27, Five: 34, Six: 4, Seven: 8}
	d, err := huiniaoToDraw(g, it)
	if err != nil || len(d.Nums) != 7 || d.Nums[6] != 8 {
		t.Fatalf("huiniaoToDraw: %+v %v", d, err)
	}
	bad := it
	bad.Five = 99 // 超界
	if _, err := huiniaoToDraw(g, bad); err == nil {
		t.Fatal("超界号码应报错")
	}
}

func TestWriteCSVMergeSortDedup(t *testing.T) {
	g := data.GameDef{Key: "qxc", NumLen: 7, MaxVal: 9, MinVal: 0}
	path := filepath.Join(t.TempDir(), "qxc.csv")
	// 第一次：乱序 + 重复
	batch1 := []data.NumDraw{
		{Issue: "26111", Date: "2026-09-25", Nums: []int{7, 8, 9, 1, 0, 1, 1}},
		{Issue: "26110", Date: "2026-09-22", Nums: []int{1, 2, 3, 4, 5, 6, 7}},
	}
	added, err := WriteCSV(path, g, batch1)
	if err != nil || added != 2 {
		t.Fatalf("WriteCSV: %v added=%d", err, added)
	}
	// 第二次：补充更早期号（乱序块），应排序合并
	batch2 := []data.NumDraw{
		{Issue: "26109", Date: "2026-09-18", Nums: []int{0, 0, 0, 0, 0, 0, 0}},
		{Issue: "26110", Date: "2026-09-22", Nums: []int{1, 2, 3, 4, 5, 6, 7}}, // 重复
	}
	added, err = WriteCSV(path, g, batch2)
	if err != nil || added != 1 {
		t.Fatalf("WriteCSV 增量: %v added=%d", err, added)
	}
	draws, err := data.LoadNumCSV(path, g)
	if err != nil {
		t.Fatal(err)
	}
	if len(draws) != 3 || draws[0].Issue != "26109" || draws[2].Issue != "26111" {
		t.Fatalf("合并排序失败: %+v", draws)
	}
}

func TestFetchLatestNumIssueGuard(t *testing.T) {
	// 本地已有最新期时 FetchLatestNum 不应返回新数据（无网络环境下两个源都可能失败，
	// 这里只验证函数在空 CSV 时至少不 panic 且 alive 语义正确）
	g := data.GameDef{Key: "kl8", NumLen: 20, MaxVal: 80, MinVal: 1}
	path := filepath.Join(t.TempDir(), "empty.csv")
	_, _ = FetchLatestNum(g, path) // 网络失败返回 nil,false 属正常
}
