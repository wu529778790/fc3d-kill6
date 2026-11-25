package fetch

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"fc3d-kill6/data"
)

// 17500 全量 TXT 的两种历史格式：
//   - 旧格式 15 列（期号 日期 百 十 个 + 10 附加数字）
//   - 2026-11 起 17 列（行尾多出销售额等两列），曾导致「取末尾 15 列」把期号读成号码
const mock17500 = `2026201 2026-07-30 3 2 1 0 0 0 0 0 0 0 0 0 0 0
2026202 2026-07-31 9 8 8 0 0 0 0 0 0 0 0 0 0 0
2026222 2026-08-20 3 8 0 0 0 0 0 0 0 0 0 0 0
`

const mock17500Wide = `2026313 2026-11-23 3 4 6 0 3 2 1 1 106369536 0 1040 0 346 0 173
2026314 2026-11-24 5 8 4 6 5 5 1 1 104678984 0 1040 0 346 0 173
`

func TestFetch17500Parse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte(mock17500))
	}))
	defer srv.Close()

	lt, err := parse17500Line([]byte(mock17500))
	if err != nil {
		t.Fatalf("parse17500Line: %v", err)
	}
	if lt.Issue != "2026222" || lt.Date != "2026-08-20" || lt.B != 3 || lt.S != 8 || lt.G != 0 {
		t.Fatalf("结果错误: %+v", lt)
	}
	_ = srv
}

// 2026-11 起 17500 行尾附加列变多，必须按行取前 5 列而不是按列数截取
func TestFetch17500ParseWideFormat(t *testing.T) {
	lt, err := parse17500Line([]byte(mock17500Wide))
	if err != nil {
		t.Fatalf("parse17500Line: %v", err)
	}
	if lt.Issue != "2026314" || lt.Date != "2026-11-24" || lt.B != 5 || lt.S != 8 || lt.G != 4 {
		t.Fatalf("宽格式解析错误: %+v", lt)
	}
}

// 补缺口：漏期（如抓数中断 10 天）必须能一次补齐，且只用大于本地最新期的记录
func TestFetchMissingGap(t *testing.T) {
	body := []byte(mock17500 + `
2026305 2026-11-15 5 2 4 0 0 1 1 1 100000 0 1040 0 346 0 173
2026306 2026-11-16 9 1 3 0 0 1 1 1 100000 0 1040 0 346 0 173
`)
	all := Parse17500All(body)
	// 升序（尽管输入有乱行顺序）
	for i := 1; i < len(all); i++ {
		if all[i].Issue < all[i-1].Issue {
			t.Fatalf("Parse17500All 未按升序: %s < %s", all[i].Issue, all[i-1].Issue)
		}
	}
	gap := missingAfter(all, "2026222")
	if len(gap) != 2 || gap[0].Issue != "2026305" || gap[1].Issue != "2026306" {
		t.Fatalf("补缺口结果错误: %+v", gap)
	}
	if gap[0].B != 5 || gap[0].S != 2 || gap[0].G != 4 {
		t.Fatalf("补缺口号码错误: %+v", gap[0])
	}
	if len(missingAfter(all, "2026999")) != 0 {
		t.Fatal("本地已是最新时不应产生补缺记录")
	}
}

// 表头/空行/脏行不能被当成开奖数据
func TestFetch17500ParseSkipsJunkTail(t *testing.T) {
	body := mock17500Wide + "\n"
	if _, err := parse17500Line([]byte(body)); err != nil {
		t.Fatalf("尾部空行不应出错: %v", err)
	}
	if _, err := parse17500Line([]byte("issue date b s g\n")); err == nil {
		t.Fatal("表头不应被解析成开奖数据")
	}
	if _, err := parse17500Line([]byte("2026314 2026-11-24\n")); err == nil {
		t.Fatal("字段不足不应被解析成功")
	}
}

func TestAppendCSVIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.csv")
	// 初始数据
	if _, err := data.AppendCSV(path, data.Draw{Issue: "2026001", Date: "2026-01-01", B: 1, S: 2, G: 3}); err != nil {
		t.Fatal(err)
	}
	// 重复追加同期号 → 返回 0
	n, err := data.AppendCSV(path, data.Draw{Issue: "2026001", Date: "2026-01-01", B: 1, S: 2, G: 3})
	if err != nil || n != 0 {
		t.Fatalf("重复追加应返回 0, got %d err=%v", n, err)
	}
	// 新期号 → 返回 1
	n, err = data.AppendCSV(path, data.Draw{Issue: "2026002", Date: "2026-01-02", B: 4, S: 5, G: 6})
	if err != nil || n != 1 {
		t.Fatalf("新期号应返回 1, got %d err=%v", n, err)
	}
	draws, err := data.LoadCSV(path)
	if err != nil || len(draws) != 2 {
		t.Fatalf("应 2 期, got %d err=%v", len(draws), err)
	}
	if draws[1].Issue != "2026002" || draws[1].B != 4 {
		t.Fatalf("第二期解析错误: %+v", draws[1])
	}
}

func TestNextIssueCalc(t *testing.T) {
	cases := []struct {
		issue, date, next string
		want              string
	}{
		{"2026365", "2026-12-31", "", "2027001"},        // 跨年回绕
		{"2026365", "2026-12-31", "2027001", "2027001"}, // 数据源优先
		{"2026009", "2026-01-09", "", "2026010"},        // 补零
		{"2026222", "2026-08-20", "2026223", "2026223"},
	}
	for _, c := range cases {
		if got := NextIssueCalc(c.issue, c.date, c.next); got != c.want {
			t.Fatalf("NextIssueCalc(%s,%s,%s)=%s, want %s", c.issue, c.date, c.next, got, c.want)
		}
	}
}

// 确保数据目录文件存在（防误删）
func TestDataDirHasCSV(t *testing.T) {
	if _, err := os.Stat(filepath.Join("..", "fc3d-history.csv")); err != nil {
		t.Skip("项目根 CSV 不存在（未在仓库根运行）")
	}
}
