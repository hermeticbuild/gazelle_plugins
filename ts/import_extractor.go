// import_extractor.go: cgo client for the in-process import-extractor library.
//
// The Rust crate at //crates/import_extractor exposes a C ABI (ie_dispatch /
// ie_free) that wraps wire::dispatch. We link the staticlib into this
// go_library and call it directly via cgo — no subprocess, no stdin/stdout
// frames, no runfiles binary lookup.
//
// Wire format is unchanged: each call marshals a pb.Request, hands the bytes
// to ie_dispatch, and unmarshals the response bytes that come back. The Rust
// side allocates the response buffer; we release it with ie_free.
package ts

/*
#include <stddef.h>
#include <stdint.h>

void ie_dispatch(
    const uint8_t *req_ptr,
    size_t req_len,
    uint8_t **out_resp_ptr,
    size_t *out_resp_len);

void ie_free(uint8_t *ptr, size_t len);
*/
import "C"

import (
	"fmt"
	"sync"
	"unsafe"

	pb "github.com/hermeticbuild/gazelle_ts/ts/proto"

	"google.golang.org/protobuf/proto"
)

// ImportExtractor is a thin handle around the cgo dispatch entry point. We
// keep the type so call sites that hold an *ImportExtractor don't change.
// All methods are serialized via the mutex because rayon parallelism inside
// Rust already saturates cores; concurrent cgo calls would just contend.
type ImportExtractor struct {
	mu     sync.Mutex
	nextID uint32
}

// newImportExtractor returns a usable handle. There's no subprocess to start
// (the Rust code is linked in), so this never fails.
func newImportExtractor() (*ImportExtractor, error) {
	return &ImportExtractor{}, nil
}

// Close is a no-op — kept so the lifecycle manager's call site is unchanged.
func (p *ImportExtractor) Close() error { return nil }

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
	resp, err := dispatch(req)
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
	default:
		return nil, fmt.Errorf("import-extractor: empty response oneof")
	}
}

// dispatch marshals req, calls into Rust, and unmarshals the response.
func dispatch(req *pb.Request) (*pb.Response, error) {
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	var reqPtr *C.uint8_t
	if len(reqBytes) > 0 {
		reqPtr = (*C.uint8_t)(unsafe.Pointer(&reqBytes[0]))
	}

	var respPtr *C.uint8_t
	var respLen C.size_t
	C.ie_dispatch(reqPtr, C.size_t(len(reqBytes)), &respPtr, &respLen)

	if respPtr == nil || respLen == 0 {
		return nil, fmt.Errorf("import-extractor: empty response from FFI")
	}
	defer C.ie_free(respPtr, respLen)

	respBytes := C.GoBytes(unsafe.Pointer(respPtr), C.int(respLen))
	var resp pb.Response
	if err := proto.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &resp, nil
}
