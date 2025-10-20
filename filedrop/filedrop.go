package filedrop

import (
	"naevis/filemgr"
	"naevis/utils"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// POST /api/v1/filedrop
func FileDropHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	paths, names, resolutions, err := HandleMediaUpload(r, "video", filemgr.EntityLoops)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "could not upload video: "+err.Error())
		return
	}

	if len(names) == 0 {
		utils.RespondWithError(w, http.StatusBadRequest, "no video uploaded")
		return
	}

	utils.RespondWithJSON(w, http.StatusCreated, map[string]any{
		"videoId":     names[0], // assuming you only expect one video
		"path":        paths[0],
		"resolutions": resolutions,
	})
}
