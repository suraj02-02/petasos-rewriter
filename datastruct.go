package main

type UpdateResourceRequest struct {
	IpAddress               string `json:"ipAddress"`
	CertificateProviderType string `json:"certificateProviderType"`
	CertificateExpiryDate   string `json:"certificateExpiryDate"`
	LastRebootReason        string `json:"lastRebootReason,omitempty"`
	WanInterfaceUsed        string `json:"wanInterfaceUsed,omitempty"`
	LastReconnectReason     string `json:"lastReconnectReason,omitempty"`
	ManagementProtocol      string `json:"managementProtocol,omitempty"`
	LastBootTime            int64  `json:"lastBootTime,omitempty"`
	FirmwareVersion         string `json:"firmwareVersion,omitempty"`
}

type WebPAConveyHeaderData struct {
	WebpaProtocol            string `json:"webpa-protocol"`
	WebpaInterfaceUsed       string `json:"webpa-interface-used"`
	HwLastRebootReason       string `json:"hw-last-reboot-reason"`
	WebpaLastReconnectReason string `json:"webpa-last-reconnect-reason"`
	BootTime                 int64  `json:"boot-time"`
	FwName                   string `json:"fw-name"`
}
