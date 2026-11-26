package engine

import (
	"fmt"
	"unicode/utf16"
	"unsafe"
)

var (
	getAnsiCodePage     = kernel32Job.NewProc("GetACP")
	wideCharToMultiByte = kernel32Job.NewProc("WideCharToMultiByte")
)

func encodeMI(command string) ([]byte, error) {
	page, _, _ := getAnsiCodePage.Call()
	if page == 65001 {
		return []byte(command), nil
	}
	wide := utf16.Encode([]rune(command))
	narrow := make([]byte, len(wide)*4)
	var usedDefault uint32
	const noBestFitChars = 0x400
	size, _, err := wideCharToMultiByte.Call(page, noBestFitChars,
		uintptr(unsafe.Pointer(&wide[0])), uintptr(len(wide)),
		uintptr(unsafe.Pointer(&narrow[0])), uintptr(len(narrow)),
		0, uintptr(unsafe.Pointer(&usedDefault)))
	if size == 0 {
		return nil, fmt.Errorf("encode GDB command in Windows code page %d: %w", page, err)
	}
	if usedDefault != 0 {
		return nil, fmt.Errorf("GDB command contains a path that Windows code page %d cannot represent", page)
	}
	return narrow[:size], nil
}
