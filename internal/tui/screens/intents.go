package screens

import "wslproxy/internal/config"

// Intent represents a typed request from a screen to the root model.
// The root coordinates navigation and service operations; screens do not perform
// those service operations themselves.
type Intent interface{ screenIntent() }

// StartProxyIntent requests starting the proxy from the menu, or opening its
// status screen if the proxy is already running.
type StartProxyIntent struct{}

// OpenFormIntent requests opening the configuration form from the menu,
// initialized with the current application configuration.
type OpenFormIntent struct{}

// ChooseDistributionIntent requests opening the WSL distribution selector
// from the menu or configuration form and loading the available distributions.
type ChooseDistributionIntent struct {
	// ForForm routes the selected distribution back to the configuration form.
	// Otherwise, the selection is saved to the application configuration.
	ForForm bool
}

// DiagnoseIntent requests running proxy diagnostics and showing their screen,
// either from the menu or when the user reruns diagnostics.
type DiagnoseIntent struct{}

// QuitIntent requests exiting the application from the menu.
type QuitIntent struct{}

// RefreshDistributionsIntent requests reloading the available WSL distributions
// when the user refreshes the selector.
type RefreshDistributionsIntent struct{}

// DistributionSelectedIntent reports a confirmed selection in the WSL
// distribution selector for the root model to apply.
type DistributionSelectedIntent struct {
	// Name identifies the WSL distribution selected by the user.
	Name string
	// ForForm updates the configuration form draft and returns to it.
	// Otherwise, the selection is saved to the application configuration.
	ForForm bool
}

// SaveConfigIntent requests saving the validated configuration submitted
// by the form; the root applies it and returns to the menu on success.
type SaveConfigIntent struct{ Config config.Config }

// ShowErrorIntent reports a form validation error for the root model to display
// without starting a save operation.
type ShowErrorIntent struct{ Err error }

// StopProxyIntent requests stopping the proxy from its status screen;
// the root returns to the menu when the operation completes.
type StopProxyIntent struct{}

func (StartProxyIntent) screenIntent()           {}
func (OpenFormIntent) screenIntent()             {}
func (ChooseDistributionIntent) screenIntent()   {}
func (DiagnoseIntent) screenIntent()             {}
func (QuitIntent) screenIntent()                 {}
func (RefreshDistributionsIntent) screenIntent() {}
func (DistributionSelectedIntent) screenIntent() {}
func (SaveConfigIntent) screenIntent()           {}
func (ShowErrorIntent) screenIntent()            {}
func (StopProxyIntent) screenIntent()            {}
