# Split func main() out of main.go into main_entry.go (!snapshot gated),
# so the snapshot build (-tags snapshot) can supply its own main.
$enc = New-Object System.Text.UTF8Encoding($false)
$src = "C:\Users\User\Documents\GitHub\RIGG\X\main.go"
$c = [System.IO.File]::ReadAllLines($src)
$idx = -1
for ($i = 0; $i -lt $c.Count; $i++) {
    if ($c[$i] -eq "func main() {") { $idx = $i; break }
}
if ($idx -lt 0) { Write-Error "func main not found"; exit 1 }
# The file previously had a build-tag-free import block; the split keeps
# main.go as-is minus func main, and writes main_entry.go with its own
# package/import header.
$tail = $c[$idx..($c.Count - 1)]
$head = $c[0..($idx - 2)]
[System.IO.File]::WriteAllLines($src, $head, $enc)
$out = New-Object System.Collections.Generic.List[string]
$out.Add("//go:build !snapshot")
$out.Add("")
$out.Add("package main")
$out.Add("")
$out.Add('import (')
$out.Add('	"fmt"')
$out.Add('	"log"')
$out.Add('')
$out.Add('	"github.com/egoist/mygo"')
$out.Add('')
$out.Add('	"github.com/OG-Open-Source/RIGG/X/internal/xapi/auth"')
$out.Add('	"github.com/OG-Open-Source/RIGG/X/internal/xapi/graphql"')
$out.Add('	"github.com/OG-Open-Source/RIGG/X/internal/xapi/xerrors"')
$out.Add(')')
$out.Add("")
foreach ($l in $tail) { $out.Add($l) }
[System.IO.File]::WriteAllLines("C:\Users\User\Documents\GitHub\RIGG\X\main_entry.go", $out, $enc)
Write-Output ("split done: main.go " + $head.Count + " lines, main_entry.go " + $tail.Count + " lines")
