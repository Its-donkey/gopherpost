package health

import (
	"fmt"
	"net/http"
)

func StartHealthServer(port string) {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "OK")
	})
	go http.ListenAndServe(":"+port, nil)
}
