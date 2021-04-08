module github.com/traefik/plugin-service

go 1.16

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/google/go-github/v32 v32.1.0
	github.com/google/uuid v1.1.2
	github.com/gorilla/mux v1.8.0
	github.com/julienschmidt/httprouter v1.3.0
	github.com/ldez/grignotin v0.4.1
	github.com/rs/zerolog v1.21.0
	github.com/stretchr/testify v1.7.0
	github.com/urfave/cli/v2 v2.3.0
	go.mongodb.org/mongo-driver v1.5.1
	go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo v0.19.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.19.0
	go.opentelemetry.io/otel v0.19.0
	go.opentelemetry.io/otel/exporters/trace/jaeger v0.19.0
	go.opentelemetry.io/otel/sdk v0.19.0
	go.opentelemetry.io/otel/trace v0.19.0
	golang.org/x/oauth2 v0.0.0-20210220000619-9bb904979d93
)
