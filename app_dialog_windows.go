//go:build windows

package main

import (
	goruntime "runtime"
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

// ─── COM helpers ─────────────────────────────────────────────────────────────

var (
	modOle32         = syscall.NewLazyDLL("ole32.dll")
	procCoInitEx     = modOle32.NewProc("CoInitializeEx")
	procCoCreateInst = modOle32.NewProc("CoCreateInstance")
	procCoTaskFree   = modOle32.NewProc("CoTaskMemFree")
)

type winGUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	// CLSID_FileOpenDialog  {DC1C5A9C-E88A-4DDE-A5A1-60F82A20AEF7}
	clsidFileOpen = winGUID{0xDC1C5A9C, 0xE88A, 0x4DDE, [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	// IID_IFileOpenDialog   {D57C7288-D4AD-4768-BE02-9D969532D960}
	iidFileOpen = winGUID{0xD57C7288, 0xD4AD, 0x4768, [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
)

// ─── IFileOpenDialog vtable ────────────────────────────────────────────────
// Total 29 slots: IUnknown(3) + IModalWindow(1) + IFileDialog(23) + IFileOpenDialog(2)

type iFileOpenDialogVtbl struct {
	// IUnknown
	QueryInterface, AddRef, Release uintptr
	// IModalWindow
	Show uintptr
	// IFileDialog (23 methods)
	SetFileTypes, SetFileTypeIndex, GetFileTypeIndex uintptr
	Advise, Unadvise                                 uintptr
	SetOptions, GetOptions                           uintptr
	SetDefaultFolder, SetFolder                      uintptr
	GetFolder, GetCurrentSelection                   uintptr
	SetFileName, GetFileName                         uintptr
	SetTitle, SetOkButtonLabel, SetFileNameLabel      uintptr
	GetResult, AddPlace                              uintptr
	SetDefaultExtension, Close                       uintptr
	SetClientGuid, ClearClientData, SetFilter        uintptr
	// IFileOpenDialog (2 methods)
	GetResults, GetSelectedItems uintptr
}

type iFileOpenDialog struct{ vtbl *iFileOpenDialogVtbl }

func (d *iFileOpenDialog) show() uintptr {
	r, _, _ := syscall.SyscallN(d.vtbl.Show, uintptr(unsafe.Pointer(d)), 0)
	return r
}
func (d *iFileOpenDialog) getOptions() uint32 {
	var v uint32
	syscall.SyscallN(d.vtbl.GetOptions, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(&v)))
	return v
}
func (d *iFileOpenDialog) setOptions(fos uint32) {
	syscall.SyscallN(d.vtbl.SetOptions, uintptr(unsafe.Pointer(d)), uintptr(fos))
}
func (d *iFileOpenDialog) setTitle(t *uint16) {
	syscall.SyscallN(d.vtbl.SetTitle, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(t)))
}
func (d *iFileOpenDialog) getResults() *iShellItemArray {
	var arr *iShellItemArray
	syscall.SyscallN(d.vtbl.GetResults, uintptr(unsafe.Pointer(d)), uintptr(unsafe.Pointer(&arr)))
	return arr
}
func (d *iFileOpenDialog) release() {
	syscall.SyscallN(d.vtbl.Release, uintptr(unsafe.Pointer(d)))
}

// ─── IShellItemArray vtable ────────────────────────────────────────────────
// Total 10 slots: IUnknown(3) + IShellItemArray(7)

type iShellItemArrayVtbl struct {
	QueryInterface, AddRef, Release            uintptr
	BindToHandler                              uintptr
	GetPropertyStore                           uintptr
	GetPropertyDescriptionList                 uintptr
	GetAttributes                              uintptr
	GetCount, GetItemAt, EnumItems             uintptr
}

type iShellItemArray struct{ vtbl *iShellItemArrayVtbl }

func (a *iShellItemArray) getCount() uint32 {
	var n uint32
	syscall.SyscallN(a.vtbl.GetCount, uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(&n)))
	return n
}
func (a *iShellItemArray) getItemAt(i uint32) *iShellItem {
	var item *iShellItem
	syscall.SyscallN(a.vtbl.GetItemAt, uintptr(unsafe.Pointer(a)), uintptr(i), uintptr(unsafe.Pointer(&item)))
	return item
}
func (a *iShellItemArray) release() {
	syscall.SyscallN(a.vtbl.Release, uintptr(unsafe.Pointer(a)))
}

// ─── IShellItem vtable ─────────────────────────────────────────────────────
// Total 8 slots: IUnknown(3) + IShellItem(5)

type iShellItemVtbl struct {
	QueryInterface, AddRef, Release uintptr
	BindToHandler, GetParent        uintptr
	GetDisplayName                  uintptr
	GetAttributes, Compare          uintptr
}

type iShellItem struct{ vtbl *iShellItemVtbl }

const sigdnFilesystemPath = uintptr(0x80058000)

func (i *iShellItem) displayName() string {
	var ptr *uint16
	r, _, _ := syscall.SyscallN(i.vtbl.GetDisplayName,
		uintptr(unsafe.Pointer(i)), sigdnFilesystemPath, uintptr(unsafe.Pointer(&ptr)))
	if r != 0 || ptr == nil {
		return ""
	}
	name := syscall.UTF16ToString((*[32768]uint16)(unsafe.Pointer(ptr))[:])
	procCoTaskFree.Call(uintptr(unsafe.Pointer(ptr)))
	return name
}
func (i *iShellItem) release() {
	syscall.SyscallN(i.vtbl.Release, uintptr(unsafe.Pointer(i)))
}

// ─── Persistent STA dialog goroutine ──────────────────────────────────────
//
// CoInitializeEx + CoCreateInstance 冷启耗时 200–500 ms。
// 通过在应用启动时建立一个持久的 OS 线程（LockOSThread）并只初始化 COM 一次，
// 后续每次调用对话框只需 CoCreateInstance + Show，延迟大幅降低。

type dlgReq struct {
	title  string
	result chan<- dlgResp
}

type dlgResp struct {
	paths []string
	err   error
}

var (
	dlgCh   chan dlgReq
	dlgOnce sync.Once
)

// initDialogThread 启动持久 STA goroutine，由 app.go startup 调用。
// 多次调用安全（sync.Once 保证只执行一次）。
func initDialogThread() {
	dlgOnce.Do(func() {
		ch := make(chan dlgReq)
		dlgCh = ch
		go func() {
			goruntime.LockOSThread() // 永久绑定到此 OS 线程
			hr, _, _ := procCoInitEx.Call(0, 2)
			if hr != 0 && hr != 1 && hr != 0x80010106 {
				// COM 初始化失败：将错误返回给所有后续请求
				for req := range ch {
					req.result <- dlgResp{nil, fmt.Errorf("CoInitializeEx: 0x%x", hr)}
				}
				return
			}
			// COM 已就绪，循环处理对话框请求
			for req := range ch {
				paths, err := showFileDialog(req.title)
				req.result <- dlgResp{paths, err}
			}
		}()
	})
}

// showFileDialog 在已初始化 COM 的 STA 线程上创建并显示 IFileOpenDialog。
func showFileDialog(title string) ([]string, error) {
	var dlg *iFileOpenDialog
	hr, _, _ := procCoCreateInst.Call(
		uintptr(unsafe.Pointer(&clsidFileOpen)),
		0, // pUnkOuter = NULL
		1, // CLSCTX_INPROC_SERVER
		uintptr(unsafe.Pointer(&iidFileOpen)),
		uintptr(unsafe.Pointer(&dlg)),
	)
	if hr != 0 || dlg == nil {
		return nil, fmt.Errorf("CoCreateInstance IFileOpenDialog: 0x%x", hr)
	}
	defer dlg.release()

	// 移除 FOS_FILEMUSTEXIST / FOS_PATHMUSTEXIST，允许选择文件夹路径
	fos := dlg.getOptions()
	fos &^= 0x00001000 // remove FOS_FILEMUSTEXIST
	fos &^= 0x00000800 // remove FOS_PATHMUSTEXIST
	fos |= 0x00000200  // add FOS_ALLOWMULTISELECT
	fos |= 0x00000008  // add FOS_NOCHANGEDIR
	dlg.setOptions(fos)

	titleW, _ := syscall.UTF16PtrFromString(title)
	dlg.setTitle(titleW)

	// 显示对话框；用户取消 → HRESULT_FROM_WIN32(ERROR_CANCELLED) = 0x800704C7
	hr = dlg.show()
	if hr == 0x800704C7 {
		return nil, nil
	}
	if hr != 0 {
		return nil, fmt.Errorf("IFileOpenDialog.Show: 0x%x", hr)
	}

	arr := dlg.getResults()
	if arr == nil {
		return nil, nil
	}
	defer arr.release()

	n := arr.getCount()
	paths := make([]string, 0, n)
	for i := uint32(0); i < n; i++ {
		item := arr.getItemAt(i)
		if item == nil {
			continue
		}
		if p := item.displayName(); p != "" {
			paths = append(paths, p)
		}
		item.release()
	}
	return paths, nil
}

// openUploadTargetsWindows 将对话框请求派发到持久 STA 线程。
func openUploadTargetsWindows(title string) ([]string, error) {
	result := make(chan dlgResp, 1)
	dlgCh <- dlgReq{title: title, result: result}
	r := <-result
	return r.paths, r.err
}

// OpenUploadTargetsDialog opens a combined file/folder picker (Windows implementation).
func (a *App) OpenUploadTargetsDialog() ([]string, error) {
	return openUploadTargetsWindows("选择要上传的文件或文件夹（单击文件夹后点「打开」可上传目录）")
}
