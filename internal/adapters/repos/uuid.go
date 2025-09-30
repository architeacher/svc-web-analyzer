package repos

import "github.com/google/uuid"

var (
	// AnalysisNamespace is the UUID V5 namespace for analysis entities
	// Generated via: uuid_generate_v5('6ba7b811-9dad-11d1-80b4-00c04fd430c8', 'svc-web-analyzer:analysis')
	AnalysisNamespace = uuid.MustParse("a8b5e5c0-7d3f-5e1a-b8c4-8f9d2a1b3c4e")

	// OutboxNamespace is the UUID V5 namespace for outbox events
	// Generated via: uuid_generate_v5('6ba7b811-9dad-11d1-80b4-00c04fd430c8', 'svc-web-analyzer:outbox')
	OutboxNamespace = uuid.MustParse("b9c6f6d1-8e4a-5f2b-c9d5-9fadab2c4d5f")
)
