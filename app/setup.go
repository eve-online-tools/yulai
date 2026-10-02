package app

// SetupStatus is what the welcome screen needs to tell whether the app can log in.
type SetupStatus struct {
	SSOConfigured bool   `json:"ssoConfigured"`
	SSOPath       string `json:"ssoPath"`
	CallbackURL   string `json:"callbackUrl"`
	Issuer        string `json:"issuer"`
	LoginURL      string `json:"loginUrl"`
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
		SSOConfigured: cfg.SSO.ClientID != "",
		SSOPath:       cfg.Source,
		CallbackURL:   cfg.SSO.CallbackURL,
		Issuer:        cfg.SSO.Issuer,
		LoginURL:      loginURL,
	}}
}
