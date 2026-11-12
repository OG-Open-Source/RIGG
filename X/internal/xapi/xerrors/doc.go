// Package xerrors 定義引擎錯誤型別集：*GraphQLError、*InvalidInputError、
// *MissingFieldError、*MaterialError、*CredentialInvalidError、
// *TransportError，攜帶 wire schema（kind／code／訊息／retryable／
// credential_impacting）與閘道錯誤文案存活清單（docs/DECISIONS.md D5）
// （規格 12.1/12.2、REQ-LOG-042/043）。
package xerrors
