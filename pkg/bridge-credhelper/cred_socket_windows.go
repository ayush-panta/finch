//go:build windows

package bridgecredhelper

// WithCredSocket is a no-op on Windows.
func WithCredSocket(finchRootPath string, fn func() error) error {
	return fn()
}