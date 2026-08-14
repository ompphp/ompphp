package main

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/openmp-capi/include
#cgo windows CFLAGS: -I${SRCDIR}/windows_compat
#cgo linux LDFLAGS: -ldl

#include <stdlib.h>
#include <string.h>
#include "ompcapi.h"

extern void OMPPHPDispatchLifecycle(int kind);
extern int OMPPHPComponentVersionPart(int part);

extern struct OMPAPI_t ompphp_api;
static bool ompphp_initialised = false;

static void ompphp_ready(void) { OMPPHPDispatchLifecycle(1); }
static void ompphp_reset(void) { OMPPHPDispatchLifecycle(2); }
static void ompphp_free(void) { OMPPHPDispatchLifecycle(3); }

#include "events_generated.h"

static bool ompphp_initialize(void) {
	if (ompphp_initialised) return true;
	ompphp_initialised = omp_initialize_capi(&ompphp_api);
	return ompphp_initialised;
}

static void* ompphp_create_component(void) {
	if (!ompphp_initialize() || !ompphp_api.Component.Create) return NULL;
	struct ComponentVersion version = {
		OMPPHPComponentVersionPart(0), OMPPHPComponentVersionPart(1),
		OMPPHPComponentVersionPart(2), 0
	};
	return ompphp_api.Component.Create(
		0x4f4d505048500001ULL, "ompphp", version,
		(void*)ompphp_ready, (void*)ompphp_reset, (void*)ompphp_free);
}

static bool ompphp_register_events(void) {
	if (!ompphp_initialised || !ompphp_api.Event.AddHandler) return false;
	return ompphp_register_generated_events();
}

static bool ompphp_log(const char* text) {
	return ompphp_api.Core.Log && ompphp_api.Core.Log(text);
}
*/
import "C"

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/ompphp/ompphp/internal/runtime"
)

var component struct {
	sync.Mutex
	runtime *runtime.Runtime
}

type capiGateway struct{}

func (capiGateway) Call(name string, arguments []any) (any, error) {
	result, handled, err := callGenerated(name, arguments)
	if handled {
		return result, err
	}
	return nil, fmt.Errorf("native function %q is not bound", name)
}

func asInt(value any) (int, bool) {
	switch v := value.(type) {
	case int64:
		return int(v), true
	case int:
		return v, true
	default:
		return 0, false
	}
}

type capiLogger struct{}

func (capiLogger) Printf(format string, args ...any) {
	message := C.CString(fmt.Sprintf("[ompphp] "+format, args...))
	defer C.free(unsafe.Pointer(message))
	C.ompphp_log(message)
}

//export ComponentEntryPoint
func ComponentEntryPoint() unsafe.Pointer { return C.ompphp_create_component() }

//export ComponentCleanup
func ComponentCleanup() { stopRuntime() }

//export OMPPHPComponentVersionPart
func OMPPHPComponentVersionPart(part C.int) C.int {
	return C.int(componentVersionPart(runtime.Version, int(part)))
}

func componentVersionPart(version string, part int) int {
	version = strings.TrimPrefix(version, "v")
	pieces := strings.SplitN(version, ".", 3)
	if part < 0 || part >= len(pieces) {
		return 0
	}
	number := strings.SplitN(pieces[part], "-", 2)[0]
	value, err := strconv.Atoi(number)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

//export OMPPHPDispatchLifecycle
func OMPPHPDispatchLifecycle(kind C.int) {
	switch int(kind) {
	case 1:
		startRuntime()
	case 2:
		resetRuntime()
	case 3:
		stopRuntime()
	}
}

func startRuntime() {
	component.Lock()
	defer component.Unlock()
	if component.runtime != nil {
		return
	}
	entry := os.Getenv("OMPPHP_ENTRY")
	if entry == "" {
		entry = "gamemode.php"
	}
	r := runtime.New(context.Background(), capiGateway{}, capiLogger{})
	if err := r.Load(entry); err != nil {
		log.Printf("[ompphp] %v", err)
		r.Close()
		return
	}
	component.runtime = r
	if !bool(C.ompphp_register_events()) {
		capiLogger{}.Printf("could not register one or more open.mp events")
	}
}
func resetRuntime() { stopRuntime(); startRuntime() }
func stopRuntime() {
	component.Lock()
	r := component.runtime
	component.runtime = nil
	component.Unlock()
	if r != nil {
		r.Close()
	}
}

func main() {}
