//go:build windows

package winhost

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const deviceNotifyCallback = 2 // DEVICE_NOTIFY_CALLBACK

var (
	powrprof                                     = windows.NewLazySystemDLL("powrprof.dll")
	procPowerRegisterSuspendResumeNotification   = powrprof.NewProc("PowerRegisterSuspendResumeNotification")
	procPowerUnregisterSuspendResumeNotification = powrprof.NewProc("PowerUnregisterSuspendResumeNotification")
)

// deviceNotifySubscribeParameters mirrors DEVICE_NOTIFY_SUBSCRIBE_PARAMETERS.
type deviceNotifySubscribeParameters struct {
	callback uintptr
	context  uintptr
}

var (
	suspendMu      sync.Mutex
	suspendHandler func()
	// windows.NewCallback slots are never freed, so the callback and the
	// parameters Windows keeps a pointer to are created once and live forever.
	suspendCallbackOnce sync.Once
	suspendParams       deviceNotifySubscribeParameters
)

func suspendCallback(_ uintptr, eventType uintptr, _ uintptr) uintptr {
	if uint32(eventType) == pbtAPMSuspend {
		suspendMu.Lock()
		handler := suspendHandler
		suspendMu.Unlock()
		if handler != nil {
			handler()
		}
	}
	return 0 // ERROR_SUCCESS
}

// registerSuspendHandler makes Windows call handler synchronously before the
// system suspends. The suspend waits until handler returns, so handler must
// bound its own run time. The returned function unregisters it.
func registerSuspendHandler(handler func()) (func(), error) {
	if err := procPowerRegisterSuspendResumeNotification.Find(); err != nil {
		return nil, err
	}
	suspendCallbackOnce.Do(func() {
		suspendParams.callback = windows.NewCallback(suspendCallback)
	})
	suspendMu.Lock()
	suspendHandler = handler
	suspendMu.Unlock()
	var registration uintptr
	r, _, _ := procPowerRegisterSuspendResumeNotification.Call(
		deviceNotifyCallback,
		uintptr(unsafe.Pointer(&suspendParams)),
		uintptr(unsafe.Pointer(&registration)),
	)
	if r != 0 {
		suspendMu.Lock()
		suspendHandler = nil
		suspendMu.Unlock()
		return nil, fmt.Errorf("PowerRegisterSuspendResumeNotification: %w", windows.Errno(r))
	}
	return func() {
		procPowerUnregisterSuspendResumeNotification.Call(registration)
		suspendMu.Lock()
		suspendHandler = nil
		suspendMu.Unlock()
	}, nil
}
