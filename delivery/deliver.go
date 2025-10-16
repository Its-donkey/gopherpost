package delivery

import (
	"fmt"

	audit "smtpserver/internal/audit"
)

var deliverFunc = Deliver

// DeliverMessage resolves the domain and attempts SMTP delivery to one of the MX hosts.
func DeliverMessage(from, to string, data []byte) error {
	domain, err := ExtractDomain(to)
	if err != nil {
		return err
	}
	mxRecords, err := ResolveMX(domain)
	if err != nil {
		audit.Log("delivery mx lookup failed for %s: %v", domain, err)
		return fmt.Errorf("MX lookup failed for %s: %w", domain, err)
	}
	if len(mxRecords) == 0 {
		audit.Log("delivery no MX records for %s", domain)
		return fmt.Errorf("MX lookup failed for %s: no MX records", domain)
	}
	var lastErr error
	for _, mx := range mxRecords {
		err = deliverFunc(mx.Host, from, to, data)
		if err == nil {
			audit.Log("delivery succeeded to %s via %s", to, mx.Host)
			return nil
		}
		audit.Log("delivery attempt to %s via %s failed: %v", to, mx.Host, err)
		lastErr = err
	}
	return fmt.Errorf("delivery failed: %w", lastErr)
}
