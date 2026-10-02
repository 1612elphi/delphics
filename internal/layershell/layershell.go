// Package layershell binds the few gtk4-layer-shell calls the bar needs.
package layershell

// #cgo pkg-config: gtk4-layer-shell-0
// #include <stdlib.h>
// #include <gtk4-layer-shell.h>
import "C"

import "unsafe"

func Supported() bool {
	return C.gtk_layer_is_supported() != 0
}

// AnchorTop turns an unrealized GtkWindow into a full-width bar on the top layer that reserves its own height.
func AnchorTop(win unsafe.Pointer, namespace string) {
	w := (*C.GtkWindow)(win)
	C.gtk_layer_init_for_window(w)
	C.gtk_layer_set_layer(w, C.GTK_LAYER_SHELL_LAYER_TOP)
	C.gtk_layer_set_anchor(w, C.GTK_LAYER_SHELL_EDGE_TOP, 1)
	C.gtk_layer_set_anchor(w, C.GTK_LAYER_SHELL_EDGE_LEFT, 1)
	C.gtk_layer_set_anchor(w, C.GTK_LAYER_SHELL_EDGE_RIGHT, 1)
	C.gtk_layer_auto_exclusive_zone_enable(w)
	ns := C.CString(namespace)
	defer C.free(unsafe.Pointer(ns))
	C.gtk_layer_set_namespace(w, ns)
}
