package ports

import "github.com/architeacher/svc-web-analyzer/internal/handlers"

type RequestHandler interface {
	handlers.ServerInterface
}
