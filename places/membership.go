package places

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// 🏋️ Membership
func GetMembership(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("GetMembership not implemented yet"))
}

func PostMembership(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("PostMembership not implemented yet"))
}

func PutMembership(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("PutMembership not implemented yet"))
}

func DeleteMembership(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("DeleteMembership not implemented yet"))
}

func PostJoinMembership(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("PostJoinMembership not implemented yet"))
}

func GetMemberships(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("GetMemberships not implemented yet"))
}
