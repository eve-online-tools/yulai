package app

// SetupStatus is what the welcome screen and checklist need to start a login.
type SetupStatus struct {
	Issuer   string `json:"issuer"`
	LoginURL string `json:"loginUrl"`
}

// SetupService reports first-run state. Config is read once at boot, so the
// status does not change while the app runs.
type SetupService struct {
	status SetupStatus
}

func (s *SetupService) ServiceName() string { return "SetupService" }

func (s *SetupService) Status() SetupStatus { return s.status }

func newSetupService(cfg *Config, loginURL string) *SetupService {
	return &SetupService{status: SetupStatus{
		Issuer:   cfg.SSO.Issuer,
		LoginURL: loginURL,
	}}
}
