# Synthetic fixtures

These hand-authored scenarios exercise frontend replay. The fixture envelope carries stdin and optional expected stdout as text, followed by one completed `run`. Only the run output stores stdout and stderr byte slices, encoded as base64. A snapshot's output offsets select prefixes of those byte slices.

`wrong-index.json` demonstrates a wrong array index, nested struct values, partial array capture, and expected versus actual output. `limited.json` demonstrates a run stopped by a capture limit. `cancelled.json` retains a selected non-prefix array element after cancellation. `compile-failed.json` contains diagnostics and an empty history. Snapshots represent selected pauses. Unknown or uncaptured elements have no recorded value. Real debugger recordings belong in separate fixtures with provenance.

`data-first-stress.json` 是合成布局夹具。每两个快照代表一组课程题型的数据形状，涵盖多矩阵、高精度数组和 18 位整数、含内部数组的结构体、缓存与递归、稀疏大下标、大数组加多标量、递归同名变量、嵌套结构体数组。这些数据用于回放布局和操作验收。
