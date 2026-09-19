//go:build !windows && !darwin

package uiapp

import "errors"

func openFolder(string) error { return errors.New("open folder is unsupported on this platform") }
func openInIDE(string, string) error {
	return errors.New("open in IDE is unsupported on this platform")
}
func pickFolder(uintptr) (string, bool, error) {
	return "", false, errors.New("folder picker is unsupported on this platform")
}
func (w *windowController) makeFrameless() error { return nil }
func (w *windowController) minimize() error      { return nil }
func (w *windowController) drag() error          { return nil }
func (w *windowController) close() error         { return nil }
