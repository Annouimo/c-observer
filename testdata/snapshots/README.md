# Synthetic fixtures

These hand-authored scenarios exercise frontend replay. The fixture envelope carries stdin and optional expected stdout as text, followed by one completed `run`. Only the run output stores stdout and stderr byte slices, encoded as base64. A snapshot's output offsets select prefixes of those byte slices.

`wrong-index.json` demonstrates a wrong array index, nested struct values, partial array capture, and expected versus actual output. `limited.json` demonstrates a run stopped by a capture limit. `cancelled.json` retains a selected non-prefix array element after cancellation. `compile-failed.json` contains diagnostics and an empty history. Snapshots represent selected pauses. Unknown or uncaptured elements have no recorded value. Real debugger recordings belong in separate fixtures with provenance.
