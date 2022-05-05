package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

// withNetIO is a convenience func for unpacking json requests (T) and packing
// json responses [U]. Note that T and U can be anything as long as it can be
// packed/unpacked as json. Also note that an empty struct for T signals that
// nothing is expected for input. It works with the following syntax:
//
//  // This will expect to recieve a string json and send back a bool resp.
//  withNetIO(w, r, func(opts string) bool {
//      fmt.Println("got:", opts)
//      // Simply return to pack- and send the response.
//      return true
//  })
//
// If T is not an empty struct (stuct{}) and cannot be decoded, then the
// rcv func will not run, this func will simply do w.WriteHeader with a
// http.StatusBadRequest, then return.
// Similarly, if U cannot be encoded, then this func will simply do a
// w.WriteHeader with http.StatusInternalServerError, then return.
func withNetIO[T, U any](
	w http.ResponseWriter,
	r *http.Request,
	rcv func(in T) (out U),
) {
	var in T

	// Only try to unpack request data if T is not empty struct.
	_, ok := any(in).(struct{})
	if !ok {
		// Read.
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// T extract.
		if err := json.Unmarshal(body, &in); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	out := rcv(in)

	// Try send back.
	b, err := json.Marshal(out)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(b)
}
