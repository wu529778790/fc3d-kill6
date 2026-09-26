// import_multi — 多彩种全量历史导入（一次性/幂等）。
//
// 数据源：
//   - 排列3/排列5/大乐透/快乐8：17500.cn 全量 TXT（data.17500.cn/{key}_asc.txt）
//   - 七星彩/七乐彩：灰鸟 API 分页拉取（500 条/页）
//
// 输出：{key}-history.csv（issue,date,v1,v2,...，升序），与运行期追加格式一致。
//
// 用法：go run ./tools/import_multi [-dir .]
package main

import (
	"flag"
	"fmt"
	"os"

	"fc3d-kill6/data"
	"fc3d-kill6/fetch"
)

func main() {
	dir := flag.String("dir", ".", "CSV 输出目录")
	flag.Parse()

	for _, g := range data.MultiGames {
		path := *dir + "/" + g.Key + "-history.csv"
		var draws []data.NumDraw
		var err error
		fmt.Printf("📥 %s (%s)...\n", g.Name, g.Key)
		if _, ok := map[string]bool{"pl3": true, "pl5": true, "dlt": true, "kl8": true}[g.Key]; ok {
			draws, err = fetch.Fetch17500Full(g)
			if err != nil {
				fmt.Printf("  ❌ 17500: %v，尝试灰鸟分页\n", err)
			}
		}
		if draws == nil {
			draws, err = fetch.FetchHuiniaoFull(g, 30)
			if err != nil {
				fmt.Printf("  ❌ 灰鸟: %v\n", err)
				continue
			}
		}
		added, err := fetch.WriteCSV(path, g, draws)
		if err != nil {
			fmt.Printf("  ❌ 写入失败: %v\n", err)
			continue
		}
		fmt.Printf("  ✅ %s: 全量 %d 条，新增 %d → %s\n", g.Name, len(draws), added, path)
	}
	fmt.Println("\n完成。")
	os.Exit(0)
}
