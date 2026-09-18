package luau

/*
#cgo CFLAGS: -I${SRCDIR}/../cgo
#cgo LDFLAGS: -L${SRCDIR}/../cgo -lluau_ast -lc++
#include "bridge.h"
#include <stdlib.h>
*/
import "C"
import (
	"encoding/json"
	"errors"
	"unsafe"
)

type parseResponse struct {
	Error *string          `json:"error"`
	Root  *json.RawMessage `json:"root"`
}

// Parse takes Luau source code and returns the parsed JSON AST string.
func Parse(source string) (string, error) {
	cSource := C.CString(source)
	defer C.free(unsafe.Pointer(cSource))

	cResult := C.LuauParseToJSON(cSource)
	if cResult == nil {
		return "", errors.New("failed to parse: null result from Luau parser")
	}
	defer C.LuauFreeString(cResult)

	jsonStr := C.GoString(cResult)

	var resp parseResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(*resp.Error)
	}

	return jsonStr, nil
}
