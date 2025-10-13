package delivery

import (
	"fmt"
)

// DeliverMessage resolves the domain and attempts SMTP delivery to one of the MX hosts.
func DeliverMessage(from, to string, data []byte) error {
	domain, err := ExtractDomain(to)
	if err != nil {
		return err
	}
	mxRecords, err := ResolveMX(domain)
	if err != nil {
		return fmt.Errorf("MX lookup failed for %s: %w", domain, err)
	}
	if len(mxRecords) == 0 {
		return fmt.Errorf("MX lookup failed for %s: no MX records", domain)
	}
	var lastErr error
	for _, mx := range mxRecords {
		err = Deliver(mx.Host, from, to, data)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return fmt.Errorf("delivery failed: %w", lastErr)
}
