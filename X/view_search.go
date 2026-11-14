// view_search：搜尋 view 的行為面（Task 10）——submitSearch 以 feedSearch
// 槽承載搜尋結果（ephemeral：新搜尋與關閉皆以「替換槽指標」歸零，見各函
// 式內的 Task 8 不變式釘注），渲染走 homeView 的 feed 路徑（槽隔離、世代
// publish 檢查沿 T6/T8 的形）；搜尋結果只寫 feeds[feedSearch]，恆不觸及
// 他 feed 的 posts/seen。product 固定 "Top"（Task 9 裁決：必填變數）；翻
// 頁屬後續任務——游標與 end 依契約留存，AtEnd 對 feedSearch 不派發
// （loadFeed 的大聲預設）。搜尋歷史不做（Non-Goals）。
package main

import (
	"context"
	"strings"
)

// searchProduct 是搜尋結果頁的 product 變數（Task 9 裁決：SearchTimeline
// 的 product 為必填變數；搜尋結果頁用 "Top"）。
const searchProduct = "Top"

// searchBackText 是搜尋結果標題列的顯式返回入口文案（Task 14）：已選
// tab 的點擊不觸發 Changed（mygo 只標記真切換），自預設 tab 開啟的搜尋
// 無 tab 返回路徑——「返回時間線」是顯式出口（closeSearch）。
const searchBackText = "返回時間線"

// submitSearch 提交搜尋 query：空（含全空白）忽略不派發；搜尋進行中
// （feedSearch 槽 busy）防抖忽略——沿 homeBusy 模式。起跑：feedSearch 槽
// 歸零＝替換槽指標（Task 8 不變式：歸零一律換新槽、非原地清空——publish
// 防護 a.feeds[kind] != f 僅對「指標替換」有意義，在途舊結果一併作廢），
// 檢視切至搜尋結果（頂列以「搜尋結果」文字標示，tabs 不新增第三枚）→
// 背景 Search → 世代＋槽雙檢查後 publish 回 UI 執行緒套用。
func (a *app) submitSearch(query string) {
	q := strings.TrimSpace(query)
	if q == "" {
		return // 空 query 提交忽略（不派發）
	}
	if a.feeds[feedSearch].busy {
		return // 防抖：搜尋進行中再提交一律忽略
	}
	// 槽歸零＝替換指標（Task 8 不變式，非原地清空）：殘餘結果與在途
	// 回寫（槽比對落空）一併作廢。
	f := &timelineState{}
	a.feeds[feedSearch] = f
	f.busy = true
	a.searchQuery = q // 最後提交值（錯誤重試＝原 query 再提交；非歷史）
	a.searchOpen = true
	sess, gen := a.sess, a.gen
	a.goSession(func(ctx context.Context) {
		res, err := sess.Search(q, searchProduct, "")
		a.publish(func() {
			// 世代與槽一致（T6/T8 形）：gen 比對擋切換後的舊帳號結果；
			// 槽指標比對擋該槽被歸零換新後的殘餘回寫。
			if gen != a.gen || a.feeds[feedSearch] != f {
				return
			}
			f.busy = false
			if err != nil {
				f.err = err // 新換（空）列表原樣；重試入口以最後 query 再提交
				return
			}
			if res == nil {
				res = &timelineResult{}
			}
			f.mergePosts(res) // 去重合併＋槽內 seen 累積（上限 seenCap）
			if res.Cursor == "" {
				f.end = true // 無游標 → 到底（契約 4 同形），不再觸發
			} else {
				f.cursor = res.Cursor
			}
		})
	})
}

// closeSearch 關閉搜尋結果（ephemeral）：檢視離開搜尋、feedSearch 槽歸
// 零。歸零＝替換槽指標（a.feeds[feedSearch] = &timelineState{}，非原地
// 清空）——publish 防護 a.feeds[kind] != f 僅擋「指標替換後」的殘餘回寫
// （Task 8 不變式釘注）；他 feed 槽（posts/seen/list）不受影響。
func (a *app) closeSearch() {
	a.searchOpen = false
	a.feeds[feedSearch] = &timelineState{}
}
