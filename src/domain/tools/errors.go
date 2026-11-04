package tools

type NotReversibleError struct{ Tool string }

func (e *NotReversibleError) Error() string { return e.Tool + " cannot be uninstalled by station" }

type ManualInstallError struct{ Tool, Hint string }

func (e *ManualInstallError) Error() string { return e.Tool + " is installed by hand" }

type AppStoreError struct{ Tool string }

func (e *AppStoreError) Error() string { return e.Tool + " is the App Store's install" }

type NoUpgradePathError struct{ Tool string }

func (e *NoUpgradePathError) Error() string { return e.Tool + " has no upgrade path" }
