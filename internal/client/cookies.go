package client

import (
	"net/http"
	"time"

	"github.com/seifreed/tupinambacli/internal/cookieheader"
)

func mergeCookieHeader(header string, cookies []*http.Cookie) string {
	vals := cookieheader.Parse(header)
	for _, c := range cookies {
		if c != nil && c.Name != "" {
			if c.MaxAge < 0 || c.MaxAge == 0 && !c.Expires.IsZero() && !c.Expires.After(time.Now()) {
				delete(vals, c.Name)
			} else {
				vals[c.Name] = c.Value
			}
		}
	}
	return cookieheader.Format(vals)
}
