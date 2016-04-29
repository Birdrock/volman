package driverhttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	cf_http_handlers "github.com/cloudfoundry-incubator/cf_http/handlers"
	"github.com/cloudfoundry-incubator/volman/voldriver"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
)

// At present, Docker ignores HTTP status codes, and requires errors to be returned in the response body.  To
// comply with this API, we will return 200 in all cases
const (
	statusInternalServerError = http.StatusOK
	statusOK                  = http.StatusOK
)

func NewHandler(logger lager.Logger, client voldriver.Driver) (http.Handler, error) {
	logger = logger.Session("server")
	logger.Info("start")
	defer logger.Info("end")
	var handlers = rata.Handlers{

		voldriver.ActivateRoute: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			logger := logger.Session("handle-activate")
			logger.Info("start")
			defer logger.Info("end")

			activateResponse := client.Activate(logger)
			// ok to eat error as we should be removing error from the Info func signature
			if activateResponse.Err != "" {
				logger.Error("failed-activating-driver", fmt.Errorf(activateResponse.Err))
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, activateResponse)
				return
			}

			logger.Debug("activate-response", lager.Data{"activation": activateResponse})
			cf_http_handlers.WriteJSONResponse(w, http.StatusOK, activateResponse)
		}),

		voldriver.GetRoute: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			logger := logger.Session("handle-get")
			logger.Info("start")
			defer logger.Info("end")

			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				logger.Error("failed-reading-get-request-body", err)
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, voldriver.MountResponse{Err: err.Error()})
				return
			}

			var getRequest voldriver.GetRequest
			if err = json.Unmarshal(body, &getRequest); err != nil {
				logger.Error("failed-unmarshalling-get-request-body", err)
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, voldriver.GetResponse{Err: err.Error()})
				return
			}

			getResponse := client.Get(logger, getRequest)
			if getResponse.Err != "" {
				logger.Error("failed-getting-volume", err, lager.Data{"volume": getRequest.Name})
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, getResponse)
				return
			}

			cf_http_handlers.WriteJSONResponse(w, statusOK, getResponse)
		}),

		voldriver.PathRoute: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			logger := logger.Session("handle-path")
			logger.Info("start")
			defer logger.Info("end")

			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				logger.Error("failed-reading-path-request-body", err)
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, voldriver.MountResponse{Err: err.Error()})
				return
			}

			var pathRequest voldriver.PathRequest
			if err = json.Unmarshal(body, &pathRequest); err != nil {
				logger.Error("failed-unmarshalling-path-request-body", err)
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, voldriver.GetResponse{Err: err.Error()})
				return
			}

			pathResponse := client.Path(logger, pathRequest)
			if pathResponse.Err != "" {
				logger.Error("failed-activating-driver", fmt.Errorf(pathResponse.Err))
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, pathResponse)
				return
			}

			cf_http_handlers.WriteJSONResponse(w, http.StatusOK, pathResponse)
		}),

		voldriver.CreateRoute: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			logger := logger.Session("handle-create")
			logger.Info("start")
			defer logger.Info("end")

			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				logger.Error("failed-reading-create-request-body", err)
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, voldriver.ErrorResponse{Err: err.Error()})
				return
			}

			var createRequest voldriver.CreateRequest
			if err = json.Unmarshal(body, &createRequest); err != nil {
				logger.Error("failed-unmarshalling-create-request-body", err)
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, voldriver.ErrorResponse{Err: err.Error()})
				return
			}

			createResponse := client.Create(logger, createRequest)
			if createResponse.Err != "" {
				logger.Error("failed-creating-volume", errors.New(createResponse.Err))
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, createResponse)
				return
			}

			cf_http_handlers.WriteJSONResponse(w, statusOK, createResponse)
		}),

		voldriver.MountRoute: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			logger := logger.Session("handle-mount")
			logger.Info("start")
			defer logger.Info("end")

			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				logger.Error("failed-reading-mount-request-body", err)
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, voldriver.MountResponse{Err: err.Error()})
				return
			}

			var mountRequest voldriver.MountRequest
			if err = json.Unmarshal(body, &mountRequest); err != nil {
				logger.Error("failed-unmarshalling-mount-request-body", err)
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, voldriver.MountResponse{Err: err.Error()})
				return
			}

			mountResponse := client.Mount(logger, mountRequest)
			if mountResponse.Err != "" {
				logger.Error("failed-mounting-volume", errors.New(mountResponse.Err), lager.Data{"volume": mountRequest.Name})
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, mountResponse)
				return
			}

			cf_http_handlers.WriteJSONResponse(w, statusOK, mountResponse)
		}),

		voldriver.UnmountRoute: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			logger := logger.Session("handle-unmount")
			logger.Info("start")
			defer logger.Info("end")

			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				logger.Error("failed-reading-unmount-request-body", err)
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, voldriver.ErrorResponse{Err: err.Error()})
				return
			}

			var unmountRequest voldriver.UnmountRequest
			if err = json.Unmarshal(body, &unmountRequest); err != nil {
				logger.Error("failed-unmarshalling-unmount-request-body", err)
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, voldriver.ErrorResponse{Err: err.Error()})
				return
			}

			unmountResponse := client.Unmount(logger, unmountRequest)
			if unmountResponse.Err != "" {
				logger.Error("failed-unmount-volume", errors.New(unmountResponse.Err), lager.Data{"volume": unmountRequest.Name})
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, unmountResponse)
				return
			}

			cf_http_handlers.WriteJSONResponse(w, statusOK, unmountResponse)
		}),

		voldriver.RemoveRoute: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			logger := logger.Session("handle-remove")
			logger.Info("start")
			defer logger.Info("end")

			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				logger.Error("failed-reading-remove-request-body", err)
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, voldriver.ErrorResponse{Err: err.Error()})
				return
			}

			var removeRequest voldriver.RemoveRequest
			if err = json.Unmarshal(body, &removeRequest); err != nil {
				logger.Error("failed-unmarshalling-unmount-request-body", err)
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, voldriver.ErrorResponse{Err: err.Error()})
				return
			}

			removeResponse := client.Remove(logger, removeRequest)
			if removeResponse.Err != "" {
				logger.Error("failed-remove-volume", errors.New(removeResponse.Err))
				cf_http_handlers.WriteJSONResponse(w, statusInternalServerError, removeResponse)
				return
			}

			cf_http_handlers.WriteJSONResponse(w, statusOK, removeResponse)
		}),
	}

	return rata.NewRouter(voldriver.Routes, handlers)
}
