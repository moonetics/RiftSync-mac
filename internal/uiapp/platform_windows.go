//go:build windows

package uiapp

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	procGetWindowLongPtr = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtr = user32.NewProc("SetWindowLongPtrW")
	procSetWindowPos     = user32.NewProc("SetWindowPos")
	procShowWindow       = user32.NewProc("ShowWindow")
	procReleaseCapture   = user32.NewProc("ReleaseCapture")
	procSendMessage      = user32.NewProc("SendMessageW")
	procPostMessage      = user32.NewProc("PostMessageW")

	ole32                = windows.NewLazySystemDLL("ole32.dll")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")
	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	procCoUninitialize   = ole32.NewProc("CoUninitialize")
	procCoTaskMemFree    = ole32.NewProc("CoTaskMemFree")
)

const (
	gwlStyle           = ^uintptr(15)
	wsCaption          = uintptr(0x00C00000)
	swpNoSize          = uintptr(0x0001)
	swpNoMove          = uintptr(0x0002)
	swpNoZOrder        = uintptr(0x0004)
	swpFrameChanged    = uintptr(0x0020)
	swMinimize         = uintptr(6)
	wmClose            = uintptr(0x0010)
	wmNCLButtonDown    = uintptr(0x00A1)
	htCaption          = uintptr(2)
	coinitApartment    = uintptr(0x2)
	rpcEChangedMode    = uintptr(0x80010106)
	clsctxInprocServer = uintptr(0x1)
	sigdnFileSysPath   = uint32(0x80058000)
	fosPickFolders     = uint32(0x00000020)
	fosForceFileSystem = uint32(0x00000040)
	fosPathMustExist   = uint32(0x00000800)
	errorCancelled     = uintptr(0x800704C7)
)

var (
	clsidFileOpenDialog = windows.GUID{Data1: 0xDC1C5A9C, Data2: 0xE88A, Data3: 0x4DDE, Data4: [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidIFileOpenDialog  = windows.GUID{Data1: 0xD57C7288, Data2: 0xD4AD, Data3: 0x4768, Data4: [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
)

type fileOpenDialog struct{ vtbl *fileOpenDialogVtbl }

type fileOpenDialogVtbl struct {
	QueryInterface, AddRef, Release, Show                              uintptr
	SetFileTypes, SetFileTypeIndex, GetFileTypeIndex, Advise, Unadvise uintptr
	SetOptions, GetOptions, SetDefaultFolder, SetFolder, GetFolder     uintptr
	GetCurrentSelection, SetFileName, GetFileName, SetTitle            uintptr
	SetOkButtonLabel, SetFileNameLabel, GetResult                      uintptr
}

type shellItem struct{ vtbl *shellItemVtbl }

type shellItemVtbl struct {
	QueryInterface, AddRef, Release, BindToHandler, GetParent, GetDisplayName uintptr
}

func openFolder(target string) error {
	cmd := exec.Command("explorer", target)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Start()
}

func (w *windowController) makeFrameless() error {
	if w == nil || w.native == 0 {
		return errors.New("window handle is unavailable")
	}
	style, _, err := procGetWindowLongPtr.Call(w.native, gwlStyle)
	if style == 0 && err != windows.ERROR_SUCCESS {
		return err
	}
	style &^= wsCaption
	if result, _, err := procSetWindowLongPtr.Call(w.native, gwlStyle, style); result == 0 && err != windows.ERROR_SUCCESS {
		return err
	}
	procSetWindowPos.Call(w.native, 0, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoZOrder|swpFrameChanged)
	return nil
}

func (w *windowController) minimize() error {
	if w == nil || w.native == 0 {
		return errors.New("window handle is unavailable")
	}
	procShowWindow.Call(w.native, swMinimize)
	return nil
}

func (w *windowController) drag() error {
	if w == nil || w.native == 0 {
		return errors.New("window handle is unavailable")
	}
	procReleaseCapture.Call()
	procSendMessage.Call(w.native, wmNCLButtonDown, htCaption, 0)
	return nil
}

func (w *windowController) close() error {
	if w == nil || w.native == 0 {
		return errors.New("window handle is unavailable")
	}
	procPostMessage.Call(w.native, wmClose, 0, 0)
	return nil
}

func pickFolder(owner uintptr) (string, bool, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := procCoInitializeEx.Call(0, coinitApartment)
	initialized := hr == 0 || hr == 1
	if initialized {
		defer procCoUninitialize.Call()
	} else if hr != rpcEChangedMode {
		return "", false, fmt.Errorf("initialize folder picker: 0x%x", hr)
	}
	var dialog *fileOpenDialog
	hr, _, _ = procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidFileOpenDialog)), 0, clsctxInprocServer, uintptr(unsafe.Pointer(&iidIFileOpenDialog)), uintptr(unsafe.Pointer(&dialog)))
	if failedHRESULT(hr) {
		return "", false, fmt.Errorf("open folder picker: 0x%x", hr)
	}
	if dialog == nil {
		return "", false, errors.New("open folder picker: dialog unavailable")
	}
	defer dialog.release()
	options, err := dialog.getOptions()
	if err != nil {
		return "", false, err
	}
	if err := dialog.setOptions(options | fosPickFolders | fosForceFileSystem | fosPathMustExist); err != nil {
		return "", false, err
	}
	if err := dialog.setTitle("Choose RiftSync folder"); err != nil {
		return "", false, err
	}
	hr = dialog.show(owner)
	if hr == errorCancelled {
		return "", false, nil
	}
	if failedHRESULT(hr) {
		return "", false, fmt.Errorf("show folder picker: 0x%x", hr)
	}
	item, err := dialog.result()
	if err != nil {
		return "", false, err
	}
	defer item.release()
	path, err := item.fileSystemPath()
	if err != nil {
		return "", false, err
	}
	return path, true, nil
}

func failedHRESULT(hr uintptr) bool { return hr&0x80000000 != 0 }

func (d *fileOpenDialog) release() { syscall.SyscallN(d.vtbl.Release, uintptr(unsafe.Pointer(d))) }
func (d *fileOpenDialog) show(owner uintptr) uintptr {
	hr, _, _ := syscall.SyscallN(d.vtbl.Show, uintptr(unsafe.Pointer(d)), owner)
	return hr
}
func (d *fileOpenDialog) getOptions() (uint32, error) {
	var options uint32
	hr, _, _ := syscall.SyscallN(d.vtbl.GetOptions, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(&options)))
	if failedHRESULT(hr) {
		return 0, fmt.Errorf("read folder picker options: 0x%x", hr)
	}
	return options, nil
}
func (d *fileOpenDialog) setOptions(options uint32) error {
	hr, _, _ := syscall.SyscallN(d.vtbl.SetOptions, uintptr(unsafe.Pointer(d)), uintptr(options))
	if failedHRESULT(hr) {
		return fmt.Errorf("set folder picker options: 0x%x", hr)
	}
	return nil
}
func (d *fileOpenDialog) setTitle(title string) error {
	ptr, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return err
	}
	hr, _, _ := syscall.SyscallN(d.vtbl.SetTitle, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(ptr)))
	if failedHRESULT(hr) {
		return fmt.Errorf("set folder picker title: 0x%x", hr)
	}
	return nil
}
func (d *fileOpenDialog) result() (*shellItem, error) {
	var item *shellItem
	hr, _, _ := syscall.SyscallN(d.vtbl.GetResult, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(&item)))
	if failedHRESULT(hr) {
		return nil, fmt.Errorf("read selected folder: 0x%x", hr)
	}
	if item == nil {
		return nil, errors.New("folder picker returned no item")
	}
	return item, nil
}
func (i *shellItem) release() { syscall.SyscallN(i.vtbl.Release, uintptr(unsafe.Pointer(i))) }
func (i *shellItem) fileSystemPath() (string, error) {
	var path *uint16
	hr, _, _ := syscall.SyscallN(i.vtbl.GetDisplayName, uintptr(unsafe.Pointer(i)), uintptr(sigdnFileSysPath), uintptr(unsafe.Pointer(&path)))
	if failedHRESULT(hr) {
		return "", fmt.Errorf("read selected folder path: 0x%x", hr)
	}
	if path == nil {
		return "", errors.New("folder picker returned empty path")
	}
	defer procCoTaskMemFree.Call(uintptr(unsafe.Pointer(path)))
	return windows.UTF16PtrToString(path), nil
}
