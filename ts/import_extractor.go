// import_extractor.go: Go client for the import-extractor Rust subprocess.
//
// The wire format is length-prefixed protobuf frames:
//
//	[4-byte big-endian u32 length][protobuf payload of that length]
//
// Request and Response are oneof-wrapped (see crates/import-extractor/proto).
// Each call to ExtractImports sends a single TsQueryRequest variant and reads
// back exactly one TsResponseResult (or an error variant).
package ts

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"

	pb "github.com/hermeticbuild/gazelle_plugins/ts/proto"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"google.golang.org/protobuf/proto"
)

// ImportExtractor owns a long-lived import-extractor subprocess. All methods are
// serialized via the mutex because there's a single stdin/stdout pair.
type ImportExtractor struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.Reader
	mu     sync.Mutex
	nextID uint32
}

// newImportExtractor locates the binary via Bazel runfiles and starts it. Stderr
// is forwarded to ours so panics and skipped-file warnings are visible.
func newImportExtractor() (*ImportExtractor, error) {
	binPath := findImportExtractorBinary()
	if binPath == "" {
		return nil, fmt.Errorf("import-extractor binary not found in runfiles")
	}

	cmd := exec.Command(binPath)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", binPath, err)
	}

	return &ImportExtractor{cmd: cmd, stdin: stdin, stdout: stdout}, nil
}

// findImportExtractorBinary resolves the import-extractor binary path. Lookup order,
// highest priority first:
//
//  1. $IMPORT_EXTRACTOR_BIN — explicit absolute path. Use this when shipping
//     a prebuilt binary outside of Bazel (release artifact, vendored tool,
//     CI cache). A non-existent path is logged and the lookup continues.
//  2. Bazel runfiles — `gazelle_plugins/crates/import-extractor/bin`. The
//     standard path when the plugin runs under `bazel run //:gazelle` and
//     the gazelle_binary's `data` deps include the rust binary.
//  3. $PATH — looks for an `import_extractor` executable. Picks up a
//     `cargo install`-style global install or anything dropped on PATH by
//     a developer's environment manager.
//
// Returns "" if none match; callers log a warning and skip parsing rather
// than aborting the gazelle run.
func findImportExtractorBinary() string {
	if bin := os.Getenv("IMPORT_EXTRACTOR_BIN"); bin != "" {
		if _, err := os.Stat(bin); err == nil {
			return bin
		}
		log.Printf("ts: IMPORT_EXTRACTOR_BIN=%q does not exist; trying Bazel runfiles + $PATH", bin)
	}

	if bin, err := runfiles.Rlocation("gazelle_plugins/crates/import-extractor/bin"); err == nil {
		if _, err := os.Stat(bin); err == nil {
			return bin
		}
	}

	if bin, err := exec.LookPath("import_extractor"); err == nil {
		return bin
	}

	return ""
}

// Close shuts down the subprocess by closing stdin and waiting for exit.
func (p *ImportExtractor) Close() error {
	if p.stdin != nil {
		p.stdin.Close()
	}
	if p.cmd != nil {
		return p.cmd.Wait()
	}
	return nil
}

// ExtractImports sends a batch of file paths and returns parsed imports keyed
// by file path. Files that fail to parse are silently dropped by the Rust side.
func (p *ImportExtractor) ExtractImports(files []string) (map[string][]string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.nextID++
	req := &pb.Request{
		Id: p.nextID,
		Data: &pb.Request_TsQuery{
			TsQuery: &pb.TsQueryRequest{Files: files},
		},
	}
	if err := p.writeFrame(req); err != nil {
		return nil, err
	}

	resp, err := p.readFrame()
	if err != nil {
		return nil, err
	}
	switch d := resp.Data.(type) {
	case *pb.Response_Error:
		return nil, fmt.Errorf("import-extractor: %s", d.Error.Message)
	case *pb.Response_TsResult:
		out := make(map[string][]string, len(d.TsResult.Imports))
		for _, fi := range d.TsResult.Imports {
			out[fi.File] = fi.ImportPaths
		}
		return out, nil
	case *pb.Response_PyResult:
		return nil, fmt.Errorf("import-extractor: unexpected py_result for ts_query")
	default:
		return nil, fmt.Errorf("import-extractor: empty response oneof")
	}
}

func (p *ImportExtractor) writeFrame(req *pb.Request) error {
	payload, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(payload)))
	if _, err := p.stdin.Write(lenBuf[:]); err != nil {
		return fmt.Errorf("write length: %w", err)
	}
	if _, err := p.stdin.Write(payload); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}
	return nil
}

func (p *ImportExtractor) readFrame() (*pb.Response, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(p.stdout, lenBuf[:]); err != nil {
		return nil, fmt.Errorf("read response length: %w", err)
	}
	respLen := binary.BigEndian.Uint32(lenBuf[:])
	respBuf := make([]byte, respLen)
	if _, err := io.ReadFull(p.stdout, respBuf); err != nil {
		return nil, fmt.Errorf("read response payload: %w", err)
	}
	var resp pb.Response
	if err := proto.Unmarshal(respBuf, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &resp, nil
}
