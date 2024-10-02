package hello

import (
	"fmt"
	"io"
	"net/http"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
)

func init() {
	functions.HTTP("helloHTTP", helloHTTP)
}

func helloHTTP(w http.ResponseWriter, r *http.Request) {
	root_url := "https://raw.githubusercontent.com/mariandotg/blog/main/README.md"
	resp, err := http.Get(root_url)
	
	if err != nil {
		fmt.Fprint(w, "ERROR HACIENDO FETCH")
	}
	
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	fmt.Fprintf(w, string(body))
	
}

