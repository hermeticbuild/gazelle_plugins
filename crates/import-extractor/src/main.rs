// main.rs -- Long-lived subprocess for extracting imports from TypeScript and Python.
//
// Spawned by gazelle plugins and stays alive for the entire gazelle run.
// Communication is over stdin/stdout using length-prefixed protobuf frames.
//
// Frame format: [4-byte big-endian u32 length][protobuf payload of that length]
//
// All decoding/dispatch logic lives in `import_extractor::wire` so it's unit-testable.
// This file is just the I/O pump.

use import_extractor::wire;

fn main() {
    let stdin = std::io::stdin();
    let stdout = std::io::stdout();
    let _ = wire::process_stream(stdin.lock(), stdout.lock());
}
