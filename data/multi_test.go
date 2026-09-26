package data

import (
	"path/filepath"
	"testing"
)

func TestNumCSVRoundTrip(t *testing.T) {
	g := GameDef{Key: "pl3", Name: "排列3", NumLen: 3, MaxVal: 9, MinVal: 0, Digit: true}
	path := filepath.Join(t.TempDir(), "test.csv")
	draws := []NumDraw{
		{Issue: "2026001", Date: "2026-01-01", Nums: []int{1, 2, 3}},
		{Issue: "2026002", Date: "2026-01-02", Nums: []int{0, 9, 5}},
	}
	for _, d := range draws {
		if n, err := AppendNumCSV(path, g, d); err != nil || n != 1 {
			t.Fatalf("AppendNumCSV: %v n=%d", err, n)
		}
	}
	// 幂等：重复追加不生效
	if n, err := AppendNumCSV(path, g, draws[0]); err != nil || n != 0 {
		t.Fatalf("重复追加应为 0: %v n=%d", err, n)
	}
	got, err := LoadNumCSV(path, g)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Nums[1] != 9 {
		t.Fatalf("RoundTrip 失败: %+v", got)
	}
	if LastNumIssue(got) != "2026002" {
		t.Fatalf("LastNumIssue = %q", LastNumIssue(got))
	}
}

func TestAppendNumCSVValidation(t *testing.T) {
	g := GameDef{Key: "kl8", NumLen: 2, MaxVal: 80, MinVal: 1}
	path := filepath.Join(t.TempDir(), "x.csv")
	if _, err := AppendNumCSV(path, g, NumDraw{Issue: "1", Nums: []int{0, 5}}); err == nil {
		t.Fatal("号码低于 MinVal 应报错")
	}
	if _, err := AppendNumCSV(path, g, NumDraw{Issue: "1", Nums: []int{5}}); err == nil {
		t.Fatal("号码个数不符应报错")
	}
}

func TestProject3(t *testing.T) {
	in := []NumDraw{
		{Issue: "1", Nums: []int{9, 9, 1, 2, 3}},
		{Issue: "2", Nums: []int{4, 4, 7, 8, 0}},
	}
	out := Project3(in)
	if out[0].B != 1 || out[0].S != 2 || out[0].G != 3 {
		t.Fatalf("Project3 末三位错误: %+v", out[0])
	}
	if out[1].G != 0 {
		t.Fatalf("Project3 末位错误: %+v", out[1])
	}
}

func TestGameByKey(t *testing.T) {
	if g, ok := GameByKey("dlt"); !ok || g.Name != "大乐透" {
		t.Fatalf("GameByKey(dlt) = %+v %v", g, ok)
	}
	if len(MultiGames) != 6 {
		t.Fatalf("应有 6 个新彩种，实际 %d", len(MultiGames))
	}
}
