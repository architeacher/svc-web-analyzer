package ports

import "github.com/architeacher/svc-web-analyzer/internal/adapters/http/handlers"

type RequestHandler interface {
	handlers.ServerInterface
}
