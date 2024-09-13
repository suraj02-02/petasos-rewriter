package main

type UpdateResourceRequest struct {
	IpAddress               string `json:"ipAddress"`
	CertificateProviderType string `json:"certificateProviderType"`
	CertificateExpiryDate   string `json:"certificateExpiryDate"`
}
