# Plugin Service

```console
NAME:
   Plugin CLI serve - Serve HTTP

USAGE:
   Plugin CLI serve [command options] [arguments...]

DESCRIPTION:
   Launch plugin service application

OPTIONS:
   --addr value                 Addr to listen on. [$ADDR]
   --github-token value         GitHub Token [$GITHUB_TOKEN]
   --go-proxy-url value         Go Proxy URL [$GO_PROXY_URL]
   --go-proxy-username value    Go Proxy Username [$GO_PROXY_USERNAME]
   --go-proxy-password value    Go Proxy Password [$GO_PROXY_PASSWORD]
   --tracing-address value      Address to send traces (default: "jaeger.jaeger.svc.cluster.local:4318") [$TRACING_ADDRESS]
   --tracing-insecure           use HTTP instead of HTTPS (default: true) [$TRACING_INSECURE]
   --tracing-username value     Username to connect to Jaeger (default: "jaeger") [$TRACING_USERNAME]
   --tracing-password value     Password to connect to Jaeger (default: "jaeger") [$TRACING_PASSWORD]
   --tracing-probability value  Probability to send traces (default: 0) [$TRACING_PROBABILITY]
   --mongodb-uri value          MongoDB connection string (default: "mongodb://mongoadmin:secret@localhost:27017") [$MONGODB_URI]
   --mongodb-minpool value      MongoDB Min Pool Size (default: 10) [$MONGODB_MIN_POOL]
   --mongodb-maxpool value      MongoDB Max Pool Size (default: 30) [$MONGODB_MAX_POOL]
   --help, -h                   show help

```
