//! End-to-end tests for the wire protocol.
//!
//! These exercise `process_stream` with realistic byte streams: multiple frames in a
//! single read, frames split across reads, and malformed input. The unit tests in
//! `src/wire.rs` cover the per-request dispatch logic; these cover the framing layer
//! that `main.rs` is otherwise alone in exercising.

use import_extractor::wire;
use import_extractor_proto::import_extractor as pb;
use prost::Message;
use std::io::{Cursor, Read};

fn frame(msg: &impl Message) -> Vec<u8> {
    let body = msg.encode_to_vec();
    let mut out = Vec::with_capacity(4 + body.len());
    out.extend_from_slice(&(body.len() as u32).to_be_bytes());
    out.extend_from_slice(&body);
    out
}

fn ts_request(id: u32, files: &[&str]) -> pb::Request {
    pb::Request {
        id,
        data: Some(pb::request::Data::TsQuery(pb::TsQueryRequest {
            files: files.iter().map(|s| s.to_string()).collect(),
        })),
    }
}

fn parse_responses(bytes: &[u8]) -> Vec<pb::Response> {
    let mut out = Vec::new();
    let mut cur = Cursor::new(bytes);
    loop {
        let mut len_buf = [0u8; 4];
        if cur.read_exact(&mut len_buf).is_err() {
            return out;
        }
        let len = u32::from_be_bytes(len_buf) as usize;
        let mut body = vec![0u8; len];
        cur.read_exact(&mut body).expect("frame body");
        out.push(pb::Response::decode(&body[..]).expect("decode"));
    }
}

/// Reader that returns its source bytes in fixed-size chunks, exercising the
/// stream buffering logic when frames are split across reads.
struct ChunkedReader<'a> {
    src: &'a [u8],
    chunk: usize,
    pos: usize,
}

impl<'a> Read for ChunkedReader<'a> {
    fn read(&mut self, buf: &mut [u8]) -> std::io::Result<usize> {
        if self.pos >= self.src.len() {
            return Ok(0);
        }
        let n = std::cmp::min(self.chunk, std::cmp::min(buf.len(), self.src.len() - self.pos));
        buf[..n].copy_from_slice(&self.src[self.pos..self.pos + n]);
        self.pos += n;
        Ok(n)
    }
}

#[test]
fn empty_stream_terminates_cleanly() {
    let mut out = Vec::new();
    wire::process_stream(&[][..], &mut out).expect("empty stream ok");
    assert!(out.is_empty());
}

#[test]
fn single_request_roundtrip() {
    let dir = std::env::temp_dir().join("ie_integration_single");
    std::fs::create_dir_all(&dir).unwrap();
    let path = dir.join("a.ts");
    std::fs::write(&path, "import 'lib-a';").unwrap();

    let input = frame(&ts_request(1, &[&path.to_string_lossy()]));
    let mut output = Vec::new();
    wire::process_stream(&input[..], &mut output).expect("ok");

    let responses = parse_responses(&output);
    assert_eq!(responses.len(), 1);
    assert_eq!(responses[0].id, 1);
    match &responses[0].data {
        Some(pb::response::Data::TsResult(r)) => {
            assert_eq!(r.imports[0].import_paths, vec!["lib-a"]);
        }
        other => panic!("expected ts_result, got {other:?}"),
    }
}

#[test]
fn multiple_frames_in_one_read() {
    let dir = std::env::temp_dir().join("ie_integration_multi");
    std::fs::create_dir_all(&dir).unwrap();
    let a = dir.join("a.ts");
    let b = dir.join("b.ts");
    std::fs::write(&a, "import 'mod-a';").unwrap();
    std::fs::write(&b, "import 'mod-b';").unwrap();

    let mut input = Vec::new();
    input.extend_from_slice(&frame(&ts_request(1, &[&a.to_string_lossy()])));
    input.extend_from_slice(&frame(&ts_request(2, &[&b.to_string_lossy()])));
    input.extend_from_slice(&frame(&ts_request(3, &[])));

    let mut output = Vec::new();
    wire::process_stream(&input[..], &mut output).expect("ok");

    let responses = parse_responses(&output);
    assert_eq!(responses.len(), 3);
    assert_eq!(responses[0].id, 1);
    assert_eq!(responses[1].id, 2);
    assert_eq!(responses[2].id, 3);
    assert!(matches!(
        responses[0].data,
        Some(pb::response::Data::TsResult(_))
    ));
    assert!(matches!(
        responses[1].data,
        Some(pb::response::Data::TsResult(_))
    ));
}

#[test]
fn frame_split_across_reads() {
    // Single frame fed one byte at a time — exercises the partial-frame branch
    // of process_stream where stream.len() < 4 + len on each iteration.
    let req = ts_request(7, &[]);
    let bytes = frame(&req);

    let reader = ChunkedReader {
        src: &bytes,
        chunk: 1,
        pos: 0,
    };
    let mut output = Vec::new();
    wire::process_stream(reader, &mut output).expect("ok");

    let responses = parse_responses(&output);
    assert_eq!(responses.len(), 1);
    assert_eq!(responses[0].id, 7);
}

#[test]
fn length_prefix_split_across_reads() {
    // 4-byte length prefix split — exercises the stream.len() < 4 branch.
    let req = ts_request(11, &[]);
    let bytes = frame(&req);

    let reader = ChunkedReader {
        src: &bytes,
        chunk: 2,
        pos: 0,
    };
    let mut output = Vec::new();
    wire::process_stream(reader, &mut output).expect("ok");

    let responses = parse_responses(&output);
    assert_eq!(responses.len(), 1);
    assert_eq!(responses[0].id, 11);
}

#[test]
fn malformed_frame_yields_error_response_and_stream_continues() {
    let dir = std::env::temp_dir().join("ie_integration_bad");
    std::fs::create_dir_all(&dir).unwrap();
    let a = dir.join("a.ts");
    std::fs::write(&a, "import 'after-error';").unwrap();

    // Frame 1: garbage bytes (won't decode as Request).
    // Frame 2: valid TS request — must still be processed.
    let mut input = Vec::new();
    let garbage = vec![0xff, 0xff, 0xff, 0xff, 0xff];
    input.extend_from_slice(&(garbage.len() as u32).to_be_bytes());
    input.extend_from_slice(&garbage);
    input.extend_from_slice(&frame(&ts_request(99, &[&a.to_string_lossy()])));

    let mut output = Vec::new();
    wire::process_stream(&input[..], &mut output).expect("ok");

    let responses = parse_responses(&output);
    assert_eq!(responses.len(), 2);
    assert_eq!(responses[0].id, 0);
    assert!(matches!(
        responses[0].data,
        Some(pb::response::Data::Error(_))
    ));
    assert_eq!(responses[1].id, 99);
    match &responses[1].data {
        Some(pb::response::Data::TsResult(r)) => {
            assert_eq!(r.imports[0].import_paths, vec!["after-error"]);
        }
        _ => panic!("expected ts_result for second frame"),
    }
}

#[test]
fn batched_files_in_single_request_processed_in_parallel() {
    // Many files in one request — parallel rayon dispatch path.
    let dir = std::env::temp_dir().join("ie_integration_batch");
    std::fs::create_dir_all(&dir).unwrap();
    let mut paths = Vec::new();
    for i in 0..16 {
        let p = dir.join(format!("f{i}.ts"));
        std::fs::write(&p, format!("import 'm-{i}';")).unwrap();
        paths.push(p.to_string_lossy().into_owned());
    }
    let path_refs: Vec<&str> = paths.iter().map(String::as_str).collect();
    let req = ts_request(1, &path_refs);

    let mut output = Vec::new();
    wire::process_stream(&frame(&req)[..], &mut output).expect("ok");

    let responses = parse_responses(&output);
    assert_eq!(responses.len(), 1);
    match &responses[0].data {
        Some(pb::response::Data::TsResult(r)) => {
            assert_eq!(r.imports.len(), 16);
            for entry in &r.imports {
                assert_eq!(entry.import_paths.len(), 1);
            }
        }
        _ => panic!("expected ts_result"),
    }
}

#[test]
fn unreadable_files_skipped_quietly() {
    // process_stream must not abort when individual files fail to read; the
    // response just contains successfully-parsed files.
    let req = ts_request(
        4,
        &[
            "/this/path/definitely/does/not/exist/x.ts",
            "/another/missing/file.ts",
        ],
    );
    let mut output = Vec::new();
    wire::process_stream(&frame(&req)[..], &mut output).expect("ok");

    let responses = parse_responses(&output);
    assert_eq!(responses.len(), 1);
    match &responses[0].data {
        Some(pb::response::Data::TsResult(r)) => assert!(r.imports.is_empty()),
        _ => panic!("expected ts_result"),
    }
}
