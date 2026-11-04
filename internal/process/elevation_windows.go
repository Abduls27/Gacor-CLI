package process

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	shell32        = syscall.NewLazyDLL("shell32.dll")
	shellExecuteEx = shell32.NewProc("ShellExecuteExW")
)

type shellExecuteInfo struct {
	cbSize         uint32
	fMask          uint32
	hwnd           uintptr
	lpVerb         *uint16
	lpFile         *uint16
	lpParameters   *uint16
	lpDirectory    *uint16
	nShow          int32
	hInstApp       uintptr
	lpIDList       uintptr
	lpClass        *uint16
	hkeyClass      uintptr
	dwHotKey       uint32
	hIconOrMonitor uintptr
	hProcess       uintptr
}

// IsAdmin checks if the current process has admin privileges
func IsAdmin() bool {
	_, err := os.Open(`\\.\PHYSICALDRIVE0`)
	return err == nil
}

// RelaunchAsAdmin re-executes the current binary with "runas" verb (triggers UAC)
// and passes "--elevated" flag so the new process knows it's already admin.
func RelaunchAsAdmin() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	params, _ := syscall.UTF16PtrFromString("run --elevated")

	info := &shellExecuteInfo{
    lpVerb:       verb,
    lpFile:       file,
    lpParameters: params,
    nShow:        0, // SW_HIDE — jangan tampilkan window
	}
	info.cbSize = uint32(unsafe.Sizeof(*info))

	ret, _, _ := shellExecuteEx.Call(uintptr(unsafe.Pointer(info)))
	if ret == 0 {
		return syscall.GetLastError()
	}
	return nil
}


func RelaunchAsAdminWithArgs(args string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	params, _ := syscall.UTF16PtrFromString(args)

	info := &shellExecuteInfo{
		lpVerb:       verb,
		lpFile:       file,
		lpParameters: params,
		nShow:        0,
	}
	info.cbSize = uint32(unsafe.Sizeof(*info))

	ret, _, _ := shellExecuteEx.Call(uintptr(unsafe.Pointer(info)))
	if ret == 0 {
		return syscall.GetLastError()
	}
	return nil
}