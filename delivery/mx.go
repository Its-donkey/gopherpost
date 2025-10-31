package delivery

import (
	"fmt"
	"math/rand"
	"net"
	"sort"
	"strings"
	"time"

    "gopherpost/internal/email"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// ResolveMX returns a sorted list of MX records for a domain.
var mxLookup = net.LookupMX

func ResolveMX(domain string) ([]*net.MX, error) {
	records, err := mxLookup(domain)
	if err != nil {
		return nil, err
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].Pref < records[j].Pref
	})

	// Randomise hosts with identical preference to distribute load.
	for i := 0; i < len(records); {
		j := i + 1
		for j < len(records) && records[j].Pref == records[i].Pref {
			j++
		}
		rand.Shuffle(j-i, func(a, b int) {
			records[i+a], records[i+b] = records[i+b], records[i+a]
		})
		i = j
	}

	for _, mx := range records {
		mx.Host = strings.TrimSuffix(mx.Host, ".")
	}

	return records, nil
}

// ExtractDomain extracts the domain part from an email address.
func ExtractDomain(address string) (string, error) {
	domain, err := email.Domain(address)
	if err != nil {
		return "", fmt.Errorf("invalid email format: %w", err)
	}
	return domain, nil
}
