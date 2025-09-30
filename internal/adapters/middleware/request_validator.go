package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/architeacher/svc-web-analyzer/internal/handlers"
	"github.com/architeacher/svc-web-analyzer/internal/infrastructure"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// ErrorHandler is called when there is an error in validation.
type ErrorHandler func(logger *infrastructure.Logger, w http.ResponseWriter, message string, statusCode int)

// RequestValidatorOptions to customize request validation, openapi3filter specified options will be passed through.
type RequestValidatorOptions struct {
	Options      openapi3filter.Options
	ErrorHandler ErrorHandler
	// SilenceServersWarning allows silencing a warning for https://github.com/deepmap/oapi-codegen/issues/882 that reports when an OpenAPI spec has `spec.Servers != nil`
	SilenceServersWarning bool
}

// OapiRequestValidatorWithOptions Creates middleware to validate request by swagger spec.
// This middleware is good for net/http either since go-chi is 100% compatible with net/http.
func OapiRequestValidatorWithOptions(
	logger *infrastructure.Logger,
	swagger *openapi3.T,
	options *RequestValidatorOptions,
) func(next http.Handler) http.Handler {
	if swagger.Servers != nil && (options == nil || options.SilenceServersWarning) {
		logger.Warn().Msg("OapiRequestValidatorWithOptions called with an OpenAPI spec that has `Servers` set")
	}

	router, err := gorillamux.NewRouter(swagger)
	if err != nil {
		panic(err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip validation for OPTIONS requests (CORS preflight)
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)

				return
			}

			if statusCode, err := validateRequest(logger, r, router, options); err != nil {
				if options != nil && options.ErrorHandler != nil {
					options.ErrorHandler(logger, w, err.Error(), statusCode)
				} else {
					http.Error(w, err.Error(), statusCode)
				}

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// This function is called from the middleware above and actually does the work of validating a request.
func validateRequest(logger *infrastructure.Logger, r *http.Request, router routers.Router, options *RequestValidatorOptions) (int, error) {
	route, pathParams, err := router.FindRoute(r)
	if err != nil {
		//nolint:wrapcheck
		return http.StatusBadRequest, err // We failed to find a matching route for the request.
	}

	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:    r,
		PathParams: pathParams,
		Route:      route,
	}

	if options != nil {
		requestValidationInput.Options = &options.Options
	}

	if err := openapi3filter.ValidateRequest(r.Context(), requestValidationInput); err != nil {
		//nolint:errorlint
		switch e := err.(type) {
		case *openapi3filter.RequestError:
			errorLines := strings.Split(e.Error(), "\n")

			return http.StatusBadRequest, errors.New(errorLines[0])
		case *openapi3filter.SecurityRequirementsError:
			//nolint:wrapcheck
			return http.StatusUnauthorized, err
		default:
			// This should never happen today, but if our upstream code changes,
			// we don't want to crash the server, so handle the unexpected error.
			logger.Error().Err(err).Msg("error validating route")

			return http.StatusInternalServerError, err
		}
	}

	return http.StatusOK, nil
}

func RequestValidationErrHandler(logger *infrastructure.Logger, w http.ResponseWriter, details string, statusCode int) {
	w.WriteHeader(statusCode)

	msg := http.StatusText(statusCode)
	timeStamp := time.Now()

	httpError := handlers.ServerError{
		StatusCode: &statusCode,
		Message:    &msg,
		Details:    &details,
		Timestamp:  &timeStamp,
	}

	errRsp, err := json.Marshal(httpError)
	if err != nil {
		logger.Error().Err(err).Msg("error marshaling http error response")
	}

	if _, err = w.Write(errRsp); err != nil {
		logger.Error().Err(err).Msg("error writing http error response")
		w.WriteHeader(http.StatusInternalServerError)
	}
}
