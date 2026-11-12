// Package xapi 是 X 引擎套件樹的根。本專案為 clean-room 重寫：行為唯一
// 依據是 XAPIS_BEHAVIOR_SPEC.md，不參考、不複製任何既有 X 客戶端的程式
// 碼、註解或組織。
//
// 邊界（硬性約束）：本套件樹及其所有子套件不得 import
// github.com/egoist/mygo——X 流量只經 transport 的單一出口，UI 職責留在
// 應用層（模組根 main 反向 import 本樹）；此禁令由 importban_test.go
// 機械強制。套件組成對應設計 §3.2：fingerprint／transport／auth／
// headers／graphql／api＋xerrors。headers 的六族標頭名稱表為「v1 裁剪、
// 非最終形態」：v1 僅輸出 GraphQL GET／POST 兩族，v2 以填表補齊其餘各族。
//
// xctid 套件視 P0 探測實驗結論可能整個不存在；本樹目前未建立，探測不過
// 時其職責由 headers 的可選空位承接。
package xapi
